package bootstrap

import (
	"context"
	"io"
	"log/slog"
	"sync"

	syncerpkg "github.com/gonotelm-lab/gonotelm/internal/application/artifact/syncer"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	flowcli "github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/event"
)

type App struct {
	closers []io.Closer
	Server  *api.Server
}

func (a *App) Close() error {
	for i := len(a.closers) - 1; i >= 0; i-- {
		if err := a.closers[i].Close(); err != nil {
			slog.Error("close error", "err", err)
		}
	}
	return nil
}

func NewApp(ctx context.Context, cfg *conf.AppConfig) (_ *App, outErr error) {
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

	infra, err := NewSharedInfra(ctx, &cfg.InfraConfig)
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
	streamTaskRepo := repository.NewStreamTaskRepository(infra.Cache.ChatMessageStreamCache)

	// ── 3. Event Bus ──

	innerBus := eventbus.NewInnerEventBus()
	outerBus := eventbus.NewOuterEventBus(infra.MQ)
	bus := eventbus.NewCompositeEventBus(innerBus, outerBus)

	// ── 4. Adapters ──

	summarizer := adapter.NewSummarizer(
		infra.LLMGateway,
		cfg.Source.ModelProvider,
		cfg.Source.Model,
	)

	_ = infra.Text2Image

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

	// ── 6. Storage gateway adapter ──

	storageGateway := &storageAdapter{store: infra.Storage}

	// ── 7. Syncer ──

	syncerCfg := syncerpkg.Config{
		PerTaskInterval: cfg.Syncer.PerTaskInterval,
		GlobalInterval:  cfg.Syncer.GlobalInterval,
		GlobalBatchSize: cfg.Syncer.GlobalBatchSize,
	}
	syncerInst := syncerpkg.NewSyncer(artifactRepo, flowClient, syncerCfg, bus)
	syncerInst.Start(ctx)
	addCloser(&syncerCloser{syncerInst})

	// ── 8. Use cases ──

	// ── 9. Event handler registration ──

	event.Init(ctx, &event.EventDeps{
		NotebookRepo:       notebookRepo,
		SourceRepo:         sourceRepo,
		SourceStorageRepo:  sourceStorageRepo,
		SourceDocRepo:      sourceDocRepo,
		ChatRepo:           chatRepo,
		MessageRepo:        messageRepo,
		ContextMessageRepo: contextMsgRepo,
		ArtifactTaskRepo:   artifactRepo,
		EventBus:           bus,
		Summarizer:         summarizer,
	})

	// ── 10. HTTP Server ──

	svr := api.NewServer(api.ServerDeps{
		NotebookRepo:       notebookRepo,
		SourceRepo:         sourceRepo,
		SourceStorageRepo:  sourceStorageRepo,
		SourceDocRepo:      sourceDocRepo,
		ChatRepo:           chatRepo,
		MessageRepo:        messageRepo,
		ContextMessageRepo: contextMsgRepo,
		StreamTaskRepo:     streamTaskRepo,
		EventBus:           bus,
		WaitGroup:          &sync.WaitGroup{},
		Gateway:            infra.LLMGateway,

		ArtifactRepo: artifactRepo,
		FlowClient:   flowClient,
		Poller:       syncerInst,
		StorageGW:    storageGateway,
	})

	return &App{closers: closers, Server: svr}, nil
}

// ── bridge types ──

type storageAdapter struct {
	store storage.Storage
}

func (a *storageAdapter) DeleteObject(ctx context.Context, key string) error {
	return a.store.DeleteObject(ctx, &storage.DeleteObjectRequest{Key: key})
}

func (a *storageAdapter) PresignGet(ctx context.Context, key string) (string, error) {
	resp, err := a.store.PresignedGetObject(ctx, &storage.PresignedGetObjectRequest{Key: key})
	if err != nil {
		return "", err
	}
	return resp.Url, nil
}

type syncerCloser struct {
	syncer *syncerpkg.Syncer
}

func (s *syncerCloser) Close() error {
	s.syncer.Shutdown(context.Background())
	return nil
}
