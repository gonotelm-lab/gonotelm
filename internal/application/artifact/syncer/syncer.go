package syncer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
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
		cfg.GlobalInterval = 5 * time.Second
	}
	if cfg.GlobalBatchSize <= 0 {
		cfg.GlobalBatchSize = 100
	}
	return cfg
}

type Syncer struct {
	repo     artifactrepo.Repository
	flow     flow.TaskClient
	cfg      Config
	wg       sync.WaitGroup
	stop     chan struct{}
	eventBus eventbus.EventBus
}

func NewSyncer(repo artifactrepo.Repository, flowc flow.TaskClient, cfg Config, eventBus eventbus.EventBus) *Syncer {
	cfg = ConfigWithDefaults(cfg)
	return &Syncer{
		repo:     repo,
		flow:     flowc,
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

func mapFlowState(state flowschema.TaskState) artifactentity.Status {
	switch state {
	case flowschema.TaskState_INITED:
		return artifactentity.StatusPending
	case flowschema.TaskState_RUNNING:
		return artifactentity.StatusRunning
	case flowschema.TaskState_DONE:
		return artifactentity.StatusCompleted
	case flowschema.TaskState_FAILED:
		return artifactentity.StatusFailed
	case flowschema.TaskState_CANCELLED:
		return artifactentity.StatusCancelled
	}
	return artifactentity.StatusPending
}
