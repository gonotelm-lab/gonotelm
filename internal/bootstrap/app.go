package bootstrap

import (
	"context"
	"io"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
)

type App struct {
	closers []io.Closer
	Server  interface{ Run() }
}

func (a *App) Close() error {
	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}
	return nil
}

func NewApp(ctx context.Context, cfg *conf.Config) (_ *App, outErr error) {
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

	infra, err := NewSharedInfra(ctx, cfg)
	if err != nil {
		return nil, err
	}
	for _, c := range infra.Closers() {
		addCloser(c)
	}

	// ── 2. Repositories ──

	notebookRepo := repository.NewNotebookRepository(infra.DB.NotebookStore, infra.DB.SourceStore)
	sourceRepo := repository.NewSourceRepository(infra.DB.SourceStore)
	sourceStorageRepo := repository.NewSourceStorageRepository(infra.Storage)
	sourceDocRepo := repository.NewSourceDocRepository(
		infra.Embedder,
		infra.VDB.SourceDocStore,
		repository.SourceDocRepositoryConfig{
			EmbedBatchSize:      cfg.Embedding.BatchSize,
			EmbedMaxConcurrency: cfg.Embedding.MaxConcurrency,
		},
	)
	chatRepo := repository.NewChatRepository(infra.DB.ChatStore)
	messageRepo := repository.NewMessageRepository(infra.DB.ChatMessageStore)
	contextMsgRepo := repository.NewContextMessageRepository(infra.Cache.ChatMessageContextCache)
artifactRepo := repository.NewArtifactRepository(infra.DB.ArtifactStore)

	// ── 3. Event Bus ──

	innerBus := eventbus.NewInnerEventBus()
	outerBus := eventbus.NewOuterEventBus(infra.MQ)
	bus := eventbus.NewCompositeEventBus(innerBus, outerBus)

	// ── 4. Adapters ──

	summarizer := adapter.NewSummarizer(infra.LLMGateway)

	// ── 5. Biz objects ──
	// TODO: Migrate biz constructors to accept database.* (NEW) types instead of dal.* (OLD) types.

	_ = infra.Text2Image

	// ── 6. Logic ──
	// TODO: Migrate biz constructors to accept database.* (NEW) types instead of dal.* (OLD) types.
	/*
	appLogic := logic.MustNewLogic(
		ctx,
		oss,
		db.NotebookStore,
		db.ArtifactTaskStore,
		db.SourceStore,
		vdb.SourceDocStore,
		llmGateway,
		embeddingGateway,
		text2imageGateway,
		mqInst,
		redisClient,
	)
	_ = appLogic
	*/

	// ── 7. Event handler registration ──

	event.Init(ctx, &event.EventDeps{
		NotebookRepo:        notebookRepo,
		SourceRepo:          sourceRepo,
		SourceStorageRepo:   sourceStorageRepo,
		SourceDocRepo:       sourceDocRepo,
		ChatRepo:            chatRepo,
		MessageRepo:         messageRepo,
		ContextMessageRepo:  contextMsgRepo,
		ArtifactTaskRepo:    artifactRepo,
		EventBus:            bus,
		Summarizer:          summarizer,
	})

	// ── 8. HTTP Server ──
	// TODO: Update api.NewServer to accept explicit params instead of *infra.Instances + *wire.Wire.
	// See Tasks 9-12.

	return &App{closers: closers, Server: &dummyServer{}}, nil
}

type dummyServer struct{}

func (d *dummyServer) Run() {}
