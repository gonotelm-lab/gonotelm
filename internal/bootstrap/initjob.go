package bootstrap

import (
	"context"
	"log/slog"
	"os"

	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/dep"
	"github.com/gonotelm-lab/gonotelm/internal/application/initjob/task"
	bootshared "github.com/gonotelm-lab/gonotelm/internal/bootstrap/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/initjob"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"
)

// InitJob is the one-shot data initialization process. It wires only the
// infrastructure the registered tasks need, not the whole stack.
type InitJob struct {
	shared *bootshared.Infra
	runner *initjob.Runner
}

func NewInitJob(ctx context.Context, cfg *conf.InitJobConfig, assetsDir string) (*InitJob, error) {
	if err := trace.Init(ctx, cfg.OtelTrace); err != nil {
		slog.ErrorContext(ctx, "can not init trace", "err", err)
	}

	infra, err := bootshared.NewInfraWith(ctx, &cfg.InfraConfig,
		bootshared.ComponentDatabase,
		bootshared.ComponentRedis,
		bootshared.ComponentStorage,
	)
	if err != nil {
		return nil, err
	}

	infras := &dep.Infra{
		Database:    infra.Database,
		Redis:       infra.Redis,
		ObjectStore: infra.ObjectStore,
		KeyFactory:  infra.KeyFactory,
		Cfg:         cfg,
	}
	dependencies := &dep.Dependency{
		StylePreviewRepo: repository.NewStylePreviewRepository(
			infra.Database.ArtifactStylePreviewStore,
			nil,
		),
	}

	recorder := repository.NewInitJobRecorder(infra.Database.InitJobStore)
	runner, err := initjob.NewRunner(
		task.New(&task.Option{
			AssetsDir:  assetsDir,
			Infra:      infras,
			Dependency: dependencies,
		}),
		recorder,
		initjob.Options{
			Mode:         cfg.Mode(),
			ForceTaskIDs: cfg.ForceTaskIDs(),
			Pod:          podName(),
		},
	)
	if err != nil {
		_ = infra.Close(ctx)
		return nil, err
	}

	return &InitJob{shared: infra, runner: runner}, nil
}

// Run executes the tasks once and returns; it never waits on ctx.
func (a *InitJob) Run(ctx context.Context) error {
	return a.runner.Run(ctx)
}

func (a *InitJob) Close(ctx context.Context) error {
	if a.shared == nil {
		return nil
	}
	return a.shared.Close(ctx)
}

func podName() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
