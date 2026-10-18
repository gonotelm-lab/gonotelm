package syncer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
)

type Config struct {
	PerTaskInterval time.Duration
	GlobalInterval  time.Duration
	GlobalBatchSize int
}

func ConfigWithDefaults(cfg Config) Config {
	if cfg.PerTaskInterval <= 0 {
		cfg.PerTaskInterval = 2 * time.Second
	}
	if cfg.GlobalInterval <= 0 {
		cfg.GlobalInterval = 10 * time.Second
	}
	if cfg.GlobalBatchSize <= 0 {
		cfg.GlobalBatchSize = 100
	}
	return cfg
}

type Syncer struct {
	repo     artifactrepo.Repository
	flow     flow.TaskClient
	storage  adapter.ObjectStore
	cfg      Config
	wg       sync.WaitGroup
	stop     chan struct{}
	eventBus eventbus.Publisher
}

func New(
	repo artifactrepo.Repository,
	flowc flow.TaskClient,
	storage adapter.ObjectStore,
	cfg Config,
	eventBus eventbus.Publisher,
) *Syncer {
	cfg = ConfigWithDefaults(cfg)
	return &Syncer{
		repo:     repo,
		flow:     flowc,
		storage:  storage,
		cfg:      cfg,
		stop:     make(chan struct{}),
		eventBus: eventBus,
	}
}

func (s *Syncer) Start(ctx context.Context) {
	s.wg.Add(1)
	go s.globalLoop(ctx)
}

func (s *Syncer) Shutdown(ctx context.Context) {
	slog.InfoContext(ctx, "syncer shutting down")
	close(s.stop)
	done := make(chan struct{})
	go func() { s.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
	}
}

func mapFlowState(state flow.TaskState) artifactentity.Status {
	switch state {
	case flow.TaskStateInited:
		return artifactentity.StatusPending
	case flow.TaskStateRunning:
		return artifactentity.StatusRunning
	case flow.TaskStateDone:
		return artifactentity.StatusCompleted
	case flow.TaskStateFailed:
		return artifactentity.StatusFailed
	case flow.TaskStateCancelled:
		return artifactentity.StatusCancelled
	}
	return artifactentity.StatusPending
}
