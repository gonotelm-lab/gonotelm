package flow

import (
	"context"
	"time"

	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	flowtask "github.com/gonotelm-lab/flow/client/task"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TaskState = flowschema.TaskState

type TaskInfo struct {
	ID     string
	State  TaskState
	Result []byte
	Error  []byte
}

type TaskClient interface {
	Submit(ctx context.Context, taskType string, payload []byte) (flowTaskId string, err error)
	Get(ctx context.Context, flowTaskId string) (*TaskInfo, error)
	Cancel(ctx context.Context, flowTaskId string) error
	Close() error
}

type TaskClientImpl struct {
	client    *flowtask.Client
	namespace string
	maxRetry  int
}

func NewTaskClient(addr, namespace string, dialTimeout time.Duration, maxRetry int) (*TaskClientImpl, error) {
	_ = dialTimeout
	c, err := flowtask.New(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &TaskClientImpl{client: c, namespace: namespace, maxRetry: maxRetry}, nil
}

func (t *TaskClientImpl) Submit(ctx context.Context, taskType string, payload []byte) (string, error) {
	opts := []flowtask.SubmitOption{}
	if t.maxRetry > 0 {
		opts = append(opts, flowtask.WithMaxRetry(t.maxRetry))
	}
	task, err := t.client.Submit(ctx, t.namespace, taskType, payload, opts...)
	if err != nil {
		return "", err
	}
	return task.Id, nil
}

func (t *TaskClientImpl) Get(ctx context.Context, flowTaskId string) (*TaskInfo, error) {
	tk, err := t.client.Get(ctx, flowTaskId)
	if err != nil {
		return nil, err
	}
	return &TaskInfo{ID: tk.Id, State: tk.State, Result: tk.Result, Error: tk.Error}, nil
}

func (t *TaskClientImpl) Cancel(ctx context.Context, flowTaskId string) error {
	return t.client.Cancel(ctx, flowTaskId)
}

func (t *TaskClientImpl) Close() error { return t.client.Close() }