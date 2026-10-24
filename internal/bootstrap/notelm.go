package bootstrap

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	artifactsyncer "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact/syncer"
	chatsuggest "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/suggestion"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	bootshared "github.com/gonotelm-lab/gonotelm/internal/bootstrap/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	coreadapter "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identityservice "github.com/gonotelm-lab/gonotelm/internal/domain/identity/service"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	flowcli "github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	notelmapi "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm"
	eventnotelm "github.com/gonotelm-lab/gonotelm/internal/interfaces/event/notelm"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	"github.com/gonotelm-lab/gonotelm/pkg/mail"
	maillog "github.com/gonotelm-lab/gonotelm/pkg/mail/log"
	mailresend "github.com/gonotelm-lab/gonotelm/pkg/mail/resend"
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
	notebookRepo := repository.NewNotebookRepository(infra.Database.NotebookStore, infra.Database.NotebookMemberStore, infra.Database.SourceStore)
	notebookMemberRepo := repository.NewNotebookMemberRepository(infra.Database.NotebookMemberStore)
	notebookAccessService := notebookservice.NewAccessService(notebookRepo, notebookMemberRepo)
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
	notebookAIDescRepo := repository.NewNotebookAIDescriptionRepository(infra.Cache.NotebookAIDescriptionCache)
	providerRegistry, err := repository.NewProviderRegistry(rootCtx, cfg.ToIDPConfig())
	if err != nil {
		return nil, err
	}
	oauthStateRepo := repository.NewOAuthStateRepository(infra.Cache.OAuthStateCache)
	userRepo := repository.NewUserRepository(
		infra.Database.UserStore,
		infra.Database.UserIdentityStore,
		infra.Database.Transactor,
		infra.Cache.UserCache,
	)
	userIdentityRepo := repository.NewUserIdentityRepository(infra.Database.UserIdentityStore)
	signupRepo := repository.NewSignupRepository(infra.Cache.SignupCache)
	passwordHasher := adapter.NewBcryptHasher()
	userService := identityservice.NewUserService(
		userRepo,
		userIdentityRepo,
		infra.DistLock,
		infra.KeyFactory,
		infra.ObjectStore,
		passwordHasher,
	)
	signupService := identityservice.NewSignupService(signupRepo, userRepo, userService, passwordHasher)
	passwordResetRepo := repository.NewPasswordResetRepository(infra.Cache.PasswordResetCache)
	passwordResetService := identityservice.NewPasswordResetService(passwordResetRepo, userRepo, userService)
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
	mailer := newMailer(rootCtx, cfg.Mailer)
	// 身份域的两类事务邮件端口共用这个实现：内部选模板，发送交给通用的 mailer
	identityMailer := adapter.NewIdentityMailer(mailer)

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
	flowCfg := flowcli.Config{
		Addr:        cfg.Flow.Addr,
		Namespace:   cfg.Flow.Namespace,
		APIKey:      cfg.Flow.APIKey,
		MaxRetry:    cfg.Flow.MaxRetry,
		DialTimeout: cfg.Flow.DialTimeout,
	}

	flowTaskClient, err := flowcli.NewTaskClient(flowCfg)
	if err != nil {
		return nil, err
	}
	addCloser(flowTaskClient)

	flowSchedClient, err := flowcli.NewScheduleClient(flowCfg)
	if err != nil {
		return nil, err
	}
	addCloser(flowSchedClient)

	cleaner := cleanup.New(
		cleanup.Config{
			CallbackBaseURL: cfg.Cleanup.BaseURL,
			Delay:           cfg.Cleanup.Delay,
			Jitter:          cfg.Cleanup.Jitter,
		},
		cleanup.Deps{
			ScheduleClient: flowSchedClient,
			SourceRepo:     sourceRepo,
			NotebookRepo:   notebookRepo,
			ArtifactRepo:   artifactRepo,
			SourceDocRepo:  sourceDocRepo,
			ObjectStore:    infra.ObjectStore,
		},
	)

	// ── 7. Syncer ──
	syncerCfg := artifactsyncer.Config{
		PerTaskInterval: cfg.Syncer.PerTaskInterval,
		GlobalInterval:  cfg.Syncer.GlobalInterval,
		GlobalBatchSize: cfg.Syncer.GlobalBatchSize,
	}
	syncerInstance := artifactsyncer.New(artifactRepo, flowTaskClient, infra.ObjectStore, syncerCfg, eventBus)
	syncerInstance.Start(rootCtx)
	addCloser(&syncerCloser{syncerInstance})

	// ── 8. Event handler registration ──
	eventnotelm.Init(rootCtx, &eventnotelm.Deps{
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
		FlowClient:  flowTaskClient,

		Cleaner: cleaner,
	})

	// ── 9. HTTP Server ──
	wg := &sync.WaitGroup{}
	svr := notelmapi.NewServer(
		notelmapi.ServerDeps{
			RootCtx: rootCtx,

			NotebookRepo:              notebookRepo,
			NotebookMemberRepo:        notebookMemberRepo,
			NotebookAccessService:     notebookAccessService,
			NotebookAIDescriptionRepo: notebookAIDescRepo,

			SourceRepo:    sourceRepo,
			SourceDocRepo: sourceDocRepo,

			ObjectStore: infra.ObjectStore,
			KeyFactory:  infra.KeyFactory,

			ChatRepo:               chatRepo,
			ChatMessageRepo:        messageRepo,
			ChatContextMessageRepo: contextMsgRepo,
			ChatStreamTaskRepo:     streamTaskRepo,
			ChatSuggestionRepo:     suggestionRepo,
			ChatSuggestService:     suggestionService,

			ArtifactRepo: artifactRepo,

			StylePreviewRepo: stylePreviewRepo,
			ProviderRegistry: providerRegistry,

			OAuthStateRepo:       oauthStateRepo,
			UserRepo:             userRepo,
			UserService:          userService,
			SignupService:        signupService,
			PasswordResetService: passwordResetService,
			UserSessionRepo:      userSessionRepo,
			VerifyMailer:         identityMailer,
			ResetMailer:          identityMailer,

			EventBus:   eventBus,
			WaitGroup:  wg,
			LLMGateway: infra.LLMGateway,
			DistLock:   infra.DistLock,
			Summarizer: summarizer,

			FlowClient: flowTaskClient,
			Poller:     syncerInstance,
			TitleMaker: titleMaker,

			Cleaner: cleaner,
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
	syncer *artifactsyncer.Syncer
}

func (s *syncerCloser) Close() error {
	s.syncer.Shutdown(context.Background())
	return nil
}

// newMailer 构造事务性邮件 sender，构造失败则用 log sender，不阻塞启动
func newMailer(ctx context.Context, cfg conf.MailerConfig) coreadapter.Mailer {
	return adapter.NewMailer(newMailSender(ctx, cfg))
}

func newMailSender(ctx context.Context, cfg conf.MailerConfig) mail.Sender {
	switch mail.ParseType(cfg.Type) {
	case mail.TypeResend:
		sender, err := newResendSender(cfg)
		if err == nil {
			return sender
		}

		slog.ErrorContext(ctx, "init resend mailer failed, falling back to log mailer", slog.Any("err", err))
	case mail.TypeLog:
	default:
		slog.ErrorContext(ctx, "unknown mailer type, falling back to log mailer", slog.String("type", cfg.Type))
	}

	return maillog.New(cfg.Log.Level())
}

func newResendSender(cfg conf.MailerConfig) (mail.Sender, error) {
	httpClient := httpclient.NewBuilder(nil).WithTimeout(cfg.Timeout).Build()

	return mailresend.New(
		mail.Config{
			From:     cfg.Resend.From,
			FromName: cfg.FromName,
		},
		cfg.Resend.APIKey,
		mailresend.WithHTTPClient(httpClient),
	)
}
