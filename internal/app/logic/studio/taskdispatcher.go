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
	doHandle(ctx context.Context, task *model.ArtifactTask) (*taskHandleResult, error)
}

type taskDispatcher struct {
	handlers map[model.ArtifactKind]taskHandler
}

func newTaskDispatcher(handlers map[model.ArtifactKind]taskHandler) *taskDispatcher {
	return &taskDispatcher{handlers: handlers}
}

func (d *taskDispatcher) register(kind model.ArtifactKind, handler taskHandler) {
	d.handlers[kind] = handler
}

func (d *taskDispatcher) dispatch(ctx context.Context, task *model.ArtifactTask) (*taskHandleResult, error) {
	result, err := d.handlers[task.Kind].doHandle(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("dispatcher not found handler, kind=%s, err=%w", task.Kind, err)
	}

	return result, nil
}
