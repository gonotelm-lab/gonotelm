package studio

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
)

// not concurrent safe
type taskHandleResult struct {
	result     []byte
	resultKind model.ArtifactResultKind
}

type taskHandler interface {
	handle(ctx context.Context, task *model.ArtifactTask) (*taskHandleResult, error)
}

type taskDispatcher struct {
	handlers map[model.ArtifactKind]taskHandler
}

func newTaskDispatcher(handlers map[model.ArtifactKind]taskHandler) *taskDispatcher {
	return &taskDispatcher{handlers: handlers}
}

func (d *taskDispatcher) register(kind model.ArtifactKind, handler taskHandler) {
	if handler == nil {
		return
	}

	d.handlers[kind] = handler
}

func (d *taskDispatcher) dispatch(ctx context.Context, task *model.ArtifactTask) (*taskHandleResult, error) {
	handler, ok := d.handlers[task.Kind]
	if !ok {
		return nil, fmt.Errorf("dispatcher not found handler, kind=%s", task.Kind)
	}

	result, err := handler.handle(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("dispatch task failed, kind=%s, err=%w", task.Kind, err)
	}

	return result, nil
}
