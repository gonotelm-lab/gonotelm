package bootstrap

import (
	"context"
	"strings"

	flowworker "github.com/gonotelm-lab/flow/client/worker"
	artifactgeneration "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate"
	artifactprompt "github.com/gonotelm-lab/gonotelm/internal/application/artifact/prompt"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const taskTypePrefix = "artifact."

type WorkerApp struct {
	shared  *SharedInfra
	clients []*flowworker.Client
	cancel  context.CancelFunc
}

func NewWorkerApp(ctx context.Context, cfg *conf.Config) (*WorkerApp, error) {
	shared, err := NewSharedInfra(ctx, cfg)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(cfg.Flow.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		shared.Close(ctx)
		return nil, err
	}

	deps := &artifactgeneration.ServiceDeps{
		Agentize:      shared.AgentizeService,
		LLMGateway:    shared.LLMGateway,
		Text2Image:    shared.Text2Image,
		ObjectStorage: shared.Storage,
		Prompt:        artifactprompt.New("zh"),
	}

	taskTypes := []string{
		"artifact.mindmap",
		"artifact.report",
		"artifact.info_graphic",
		"artifact.audio_overview",
	}
	app := &WorkerApp{shared: shared}
	for _, taskType := range taskTypes {
		wcfg := flowworker.ConfigWithDefaults(flowworker.Config{
			Namespace:         cfg.Flow.Namespace,
			TaskType:          taskType,
			Name:              "gonotelm-worker-" + strings.TrimPrefix(taskType, taskTypePrefix),
			MaxConcurrency:    cfg.Worker.MaxConcurrency,
			HeartbeatInterval: cfg.Worker.Heartbeat,
		})
		c := flowworker.NewWithConn(conn, wcfg)
		artifactgeneration.RegisterTypedWorker(c, deps)
		app.clients = append(app.clients, c)
	}
	return app, nil
}

func (a *WorkerApp) Run(ctx context.Context) error {
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

func (a *WorkerApp) Close(ctx context.Context) error {
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

func kindFromTaskType(taskType string) artifactentity.Kind {
	return artifactentity.Kind(strings.TrimPrefix(taskType, taskTypePrefix))
}
