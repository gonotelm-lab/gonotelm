package flow

import (
	"context"

	flowtask "github.com/gonotelm-lab/flow/client/task/v2"
)

// TaskState 是 flow 任务的状态。直接沿用 SDK 的枚举：v2 起 SDK 不再对外暴露 proto 类型，
// 上层也只需要「跑到哪一步了」，不需要关心协议里的编号。
type TaskState = flowtask.State

const (
	TaskStateUnspecified = flowtask.StateUnspecified
	TaskStateInited      = flowtask.StateInited
	TaskStateRunning     = flowtask.StateRunning
	TaskStateDone        = flowtask.StateDone
	TaskStateFailed      = flowtask.StateFailed
	TaskStateCancelled   = flowtask.StateCancelled
)

type TaskInfo struct {
	ID        string
	State     TaskState
	Result    []byte
	Error     []byte
	MaxRetry  int
	AttemptNo int
}

type TaskClient interface {
	Submit(ctx context.Context, taskType string, payload []byte) (flowTaskId string, err error)
	Get(ctx context.Context, flowTaskId string) (*TaskInfo, error)
	Cancel(ctx context.Context, flowTaskId string) error
	Close() error
}

type TaskClientImpl struct {
	client   *flowtask.Client
	maxRetry int
}

// NewTaskClient 建立任务客户端：namespace 与 apiKey 在构造时由 Config 绑定，
// 之后的 Submit / Get / Cancel 都不再传。
func NewTaskClient(cfg Config) (*TaskClientImpl, error) {
	c, err := flowtask.New(cfg.Addr, flowtask.Config{Namespace: cfg.Namespace, APIKey: cfg.APIKey})
	if err != nil {
		return nil, err
	}
	return &TaskClientImpl{client: c, maxRetry: cfg.MaxRetry}, nil
}

func (t *TaskClientImpl) Submit(ctx context.Context, taskType string, payload []byte) (string, error) {
	opts := []flowtask.SubmitOption{}
	if t.maxRetry > 0 {
		opts = append(opts, flowtask.WithMaxRetry(t.maxRetry))
	}
	task, err := t.client.Submit(ctx, taskType, payload, opts...)
	if err != nil {
		return "", err
	}
	return task.ID, nil
}

func (t *TaskClientImpl) Get(ctx context.Context, flowTaskId string) (*TaskInfo, error) {
	tk, err := t.client.Get(ctx, flowTaskId)
	if err != nil {
		return nil, err
	}
	return &TaskInfo{ID: tk.ID, State: tk.State, Result: tk.Result, Error: tk.Error, MaxRetry: int(tk.MaxRetry), AttemptNo: int(tk.AttemptNo)}, nil
}

func (t *TaskClientImpl) Cancel(ctx context.Context, flowTaskId string) error {
	return t.client.Cancel(ctx, flowTaskId)
}

func (t *TaskClientImpl) Close() error { return t.client.Close() }
