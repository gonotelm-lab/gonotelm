package bootstrap

import (
	"context"
	"fmt"
	"log/slog"

	bootshared "github.com/gonotelm-lab/gonotelm/internal/bootstrap/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	eventsourcejob "github.com/gonotelm-lab/gonotelm/internal/interfaces/event/sourcejob"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

type SourceJob struct {
	shared *bootshared.Infra
	bus    *eventbus.CompositeEventBus
	cancel context.CancelFunc
}

func NewSourceJob(ctx context.Context, cfg *conf.SourceJobConfig) (*SourceJob, error) {
	if cfg.MessageQueue.Type == "" {
		return nil, fmt.Errorf("sourcejob requires messageQueue")
	}

	if err := trace.Init(ctx, cfg.OtelTrace); err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("[worker.bootstrap] can not init trace: %v", err))
	}

	infra, err := bootshared.NewInfra(ctx, &cfg.InfraConfig)
	if err != nil {
		return nil, err
	}
	if infra.MessageQueue == nil {
		_ = infra.Close(ctx)
		return nil, fmt.Errorf("sourcejob requires mq")
	}

	sourceRepo := repository.NewSourceRepository(infra.Database.SourceStore)
	sourceStorageRepo := repository.NewSourceStorageRepository(infra.Storage)
	sourceDocRepo := repository.NewSourceDocRepository(
		infra.Embedder,
		infra.VectorDatabase.SourceDocStore,
		repository.SourceDocRepositoryConfig{
			EmbedBatchSize:      cfg.Embedding.BatchSize,
			EmbedMaxConcurrency: cfg.Embedding.MaxConcurrency,
		},
	)

	inprocessBus := eventbus.NewInProcessEventBus()
	interprocessBus := eventbus.NewInterProcessEventBus(infra.MessageQueue)
	bus := eventbus.NewCompositeEventBus(inprocessBus, interprocessBus)

	summarizer := adapter.NewSummarizer(
		infra.LLMGateway,
		cfg.Source.ModelProvider,
		cfg.Source.Model,
	)

	imageInterpreter, err := adapter.NewImageInterpreter(
		infra.LLMGateway,
		cfg.Source.ImageModelProvider,
		cfg.Source.ImageModel,
	)
	if err != nil {
		return nil, err
	}

	eventsourcejob.Init(ctx, &eventsourcejob.EventDeps{
		SourceRepo:        sourceRepo,
		SourceStorageRepo: sourceStorageRepo,
		SourceDocRepo:     sourceDocRepo,
		EventBus:          bus,
		Summarizer:        summarizer,
		ImageInterpreter:  imageInterpreter,
	})

	return &SourceJob{
		shared: infra,
		bus:    bus,
	}, nil
}

func (a *SourceJob) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	defer a.cancel()

	slog.InfoContext(ctx, "sourcejob running, consuming source.preparation")
	<-ctx.Done()
	return nil
}

func (a *SourceJob) Close(ctx context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}

	var firstErr error
	if a.bus != nil {
		if err := a.bus.Close(ctx); err != nil {
			firstErr = err
		}
	}
	if a.shared != nil {
		if err := a.shared.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
