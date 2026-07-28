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
)

type SourceJob struct {
	shared *bootshared.Infra
	bus    eventbus.EventBus
	cancel context.CancelFunc
}

func NewSourceJob(ctx context.Context, cfg *conf.SourceJobConfig) (*SourceJob, error) {
	if cfg.MsgQueue.Type == "" {
		return nil, fmt.Errorf("sourcejob requires msgQueue")
	}

	infra, err := bootshared.NewInfra(ctx, &cfg.InfraConfig)
	if err != nil {
		return nil, err
	}
	if infra.MQ == nil {
		_ = infra.Close(ctx)
		return nil, fmt.Errorf("sourcejob requires mq")
	}

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

	innerBus := eventbus.NewInnerEventBus()
	outerBus := eventbus.NewOuterEventBus(infra.MQ)
	bus := eventbus.NewCompositeEventBus(innerBus, outerBus)

	summarizer := adapter.NewSummarizer(
		infra.LLMGateway,
		cfg.Source.ModelProvider,
		cfg.Source.Model,
	)

	eventsourcejob.Init(ctx, &eventsourcejob.EventDeps{
		SourceRepo:        sourceRepo,
		SourceStorageRepo: sourceStorageRepo,
		SourceDocRepo:     sourceDocRepo,
		EventBus:          bus,
		Summarizer:        summarizer,
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
