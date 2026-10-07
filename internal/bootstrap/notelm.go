package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	syncerpkg "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact/syncer"
	chatsuggest "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	bootshared "github.com/gonotelm-lab/gonotelm/internal/bootstrap/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	flowcli "github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	notelmapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm"
	eventnotelm "github.com/gonotelm-lab/gonotelm/internal/interfaces/event/notelm"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

type Notelm struct {
	rootCtx context.Context
	closers []io.Closer
	wg      *sync.WaitGroup
	Server  *notelmapi.Server
}

func (a *Notelm) Close() error {
	// 先等后台协程（chat stream / note title 等）结束，再关 DB/LLM 等依赖
	if a.wg != nil {
		a.wg.Wait()
	}

	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}

	trace.Shutdown(a.rootCtx)

	return nil
}

func NewNotelm(rootCtx context.Context, cfg *conf.NotelmConfig) (_ *Notelm, outErr error) {
	var closers []io.Closer
	addCloser := func(c io.Closer) { closers = append(closers, c) }
	defer func() {
		if outErr != nil {
			for i := len(closers) - 1; i >= 0; i-- {
				if err := closers[i].Close(); err != nil {
					slog.Error("close error", "err", err)
				}
			}
		}
	}()

	if err := trace.Init(rootCtx, cfg.OtelTrace); err != nil {
		slog.ErrorContext(rootCtx, fmt.Sprintf("[notelm.bootstrap] can not init trace: %v", err))
	}

	infra, err := bootshared.NewInfra(rootCtx, &cfg.InfraConfig)
	if err != nil {
		return nil, err
	}
	for _, c := range infra.Closers() {
		addCloser(c)
	}

	// ── 2. Repositories ──
	notebookRepo := repository.NewNotebookRepository(infra.Database.NotebookStore, infra.Database.SourceStore)
	sourceRepo := repository.NewSourceRepository(infra.Database.SourceStore)
	sourceDocRepo := repository.NewSourceDocRepository(
		infra.Embedder,
		infra.VectorDatabase.SourceDocStore,
		repository.SourceDocRepositoryConfig{
			EmbedBatchSize:      cfg.Embedding.BatchSize,
			EmbedMaxConcurrency: cfg.Embedding.MaxConcurrency,
		},
	)
	chatRepo := repository.NewChatRepository(infra.Database.ChatStore)
	messageRepo := repository.NewMessageRepository(infra.Database.ChatMessageStore)
	contextMsgRepo := repository.NewContextMessageRepository(infra.Cache.ChatMessageContextCache)
	artifactRepo := repository.NewArtifactRepository(infra.Database.ArtifactStore)
	stylePreviewRepo := repository.NewStylePreviewRepository(
		infra.Database.ArtifactStylePreviewStore,
		infra.Cache.StylePreviewCache,
	)
	streamTaskRepo := repository.NewStreamTaskRepository(infra.Cache.ChatMessageStreamCache)
	suggestionRepo := repository.NewSuggestionRepository(infra.Cache.ChatSuggestionCache)
	loginInfoRepo, err := repository.NewLoginInfoRepository(rootCtx, infra.Cache.LoginInfoCache, cfg.ToIDPConfig())
	if err != nil {
		return nil, err
	}
	userRepo := repository.NewUserRepository(infra.Database.UserStore)
	userService := identityservice.NewUserService(
		userRepo,
		infra.DistLock,
		infra.KeyFactory,
		infra.ObjectStore,
	)
	userSessionRepo := repository.NewUserSessionRepository(infra.Cache.UserSessionCache)

	// ── 3. Event Bus ──
	inprocessBus := eventbus.NewInProcessEventBus()
	interprocessBus := eventbus.NewInterProcessEventBus(infra.MessageQueue)
	eventBus := eventbus.NewCompositeEventBus(inprocessBus, interprocessBus)

	// ── 4. Adapters ──
	titleMaker := adapter.NewTitleMaker(
		infra.LLMGateway,
		cfg.Source.ModelProvider,
		cfg.Source.Model,
	)
	summarizer := adapter.NewSummarizer(
		infra.LLMGateway,
		cfg.Source.ModelProvider,
		cfg.Source.Model,
	)

	// ── 4.1 Suggestion service (单例) ──
	suggestionService := chatsuggest.NewService(
		rootCtx,
		infra.DistLock,
		suggestionRepo,
		messageRepo,
		contextMsgRepo,
		notebookRepo,
		sourceRepo,
		infra.LLMGateway,
	)

	// ── 5. Flow task client ──
	flowClient, err := flowcli.NewTaskClient(
		cfg.Flow.Addr,
		cfg.Flow.Namespace,
		cfg.Flow.DialTimeout,
		cfg.Flow.MaxRetry,
	)
	if err != nil {
		return nil, err
	}
	addCloser(flowClient)

	// ── 7. Syncer ──
	syncerCfg := syncerpkg.Config{
		PerTaskInterval: cfg.Syncer.PerTaskInterval,
		GlobalInterval:  cfg.Syncer.GlobalInterval,
		GlobalBatchSize: cfg.Syncer.GlobalBatchSize,
	}
	syncerInst := syncerpkg.NewSyncer(artifactRepo, flowClient, syncerCfg, eventBus)
	syncerInst.Start(rootCtx)
	addCloser(&syncerCloser{syncerInst})

	// ── 8. Event handler registration ──
	eventnotelm.Init(rootCtx, &eventnotelm.EventDeps{
		RootCtx: rootCtx,

		NotebookRepo: notebookRepo,

		SourceRepo:    sourceRepo,
		ObjectStore:   infra.ObjectStore,
		SourceDocRepo: sourceDocRepo,

		ChatRepo:               chatRepo,
		ChatMessageRepo:        messageRepo,
		ChatContextMessageRepo: contextMsgRepo,
		ChatSuggestionRepo:     suggestionRepo,
		ChatSuggestService:     suggestionService,

		ArtifactTaskRepo: artifactRepo,

		EventBus:    eventBus,
		ChatGateway: infra.LLMGateway,
	})

	// ── 9. HTTP Server ──
	wg := &sync.WaitGroup{}
	svr := notelmapi.NewServer(
		notelmapi.ServerDeps{
			RootCtx:                rootCtx,
			NotebookRepo:           notebookRepo,
			SourceRepo:             sourceRepo,
			ObjectStore:            infra.ObjectStore,
			KeyFactory:             infra.KeyFactory,
			SourceDocRepo:          sourceDocRepo,
			ChatRepo:               chatRepo,
			ChatMessageRepo:        messageRepo,
			ChatContextMessageRepo: contextMsgRepo,
			ChatStreamTaskRepo:     streamTaskRepo,
			ChatSuggestionRepo:     suggestionRepo,
			ChatSuggestService:     suggestionService,
			ArtifactRepo:           artifactRepo,
			StylePreviewRepo:       stylePreviewRepo,
			LoginInfoRepo:          loginInfoRepo,
			UserRepo:               userRepo,
			UserService:            userService,
			UserSessionRepo:        userSessionRepo,

			EventBus:   eventBus,
			WaitGroup:  wg,
			LLMGateway: infra.LLMGateway,
			DistLock:   infra.DistLock,
			Summarizer: summarizer,

			FlowClient: flowClient,
			Poller:     syncerInst,
			TitleMaker: titleMaker,
		},
	)

	return &Notelm{
		closers: closers,
		wg:      wg,
		Server:  svr,
		rootCtx: rootCtx,
	}, nil
}

type syncerCloser struct {
	syncer *syncerpkg.Syncer
}

func (s *syncerCloser) Close() error {
	s.syncer.Shutdown(context.Background())
	return nil
}
