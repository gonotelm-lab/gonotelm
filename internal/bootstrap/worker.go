package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	flowworker "github.com/gonotelm-lab/flow/client/worker"
	artifactgeneration "github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact"
	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	bootshared "github.com/gonotelm-lab/gonotelm/internal/bootstrap/shared"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	infrarepo "github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/trace"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const taskTypePrefix = "artifact."

var buildinTaskTypes = []string{
	"artifact.mindmap",
	"artifact.report",
	"artifact.info_graphic",
	"artifact.audio_overview",
	"artifact.flashcard",
	"artifact.quiz",
	"artifact.data_table",
	"artifact.slides",
}

type Worker struct {
	shared  *bootshared.Infra
	clients []*flowworker.Client
	cancel  context.CancelFunc
}

func NewWorker(ctx context.Context, cfg *conf.WorkerConfig) (*Worker, error) {
	shared, err := bootshared.NewInfra(ctx, &cfg.InfraConfig)
	if err != nil {
		return nil, err
	}

	if err := trace.Init(ctx, cfg.OtelTrace); err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("[sourcejob.bootstrap] can not init trace: %v", err))
	}

	conn, err := grpc.NewClient(
		cfg.Flow.Addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	)
	if err != nil {
		shared.Close(ctx)
		return nil, err
	}

	deps := &generatetypes.ServiceDeps{
		Agentize:             shared.AgentizeService,
		LLMGateway:           shared.LLMGateway,
		Text2Image:           shared.Text2Image,
		Text2Audio:           shared.Text2Audio,
		Sandbox:              shared.SandboxGateway,
		SandboxRepository:    infrarepo.NewSandboxRepository(shared.Cache.SandboxCache),
		DistLock:             shared.DistLock,
		ObjectStorage:        shared.Storage,
		CheckpointRepository: infrarepo.NewCheckpointRepository(shared.DB.WorkerCheckpointStore),
	}

	app := &Worker{shared: shared}
	for _, taskType := range buildinTaskTypes {
		wcfg := flowworker.ConfigWithDefaults(flowworker.Config{
			Namespace:         cfg.Flow.Namespace,
			TaskType:          taskType,
			Name:              "gnw-" + strings.TrimPrefix(taskType, taskTypePrefix),
			MaxConcurrency:    cfg.Worker.MaxConcurrency,
			HeartbeatInterval: cfg.Worker.Heartbeat,
		})
		c := flowworker.NewWithConn(conn, wcfg)
		artifactgeneration.RegisterTypedWorker(c, deps)
		app.clients = append(app.clients, c)
	}
	return app, nil
}

func (a *Worker) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	defer a.cancel()

	for _, c := range a.clients {
		if err := c.Start(); err != nil {
			return err
		}
	}
	<-ctx.Done()
	return nil
}

func (a *Worker) Close(ctx context.Context) error {
	if a.cancel != nil {
		a.cancel()
	}
	var firstErr error
	for _, c := range a.clients {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if err := a.shared.Close(ctx); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}
