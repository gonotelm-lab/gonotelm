package eventbus

import (
	"context"
	stderr "errors"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// CompositeEventBus routes publish by event category:
//   - inprocess -> in-process handlers (notebook events, source.deleted, source.indexed, ...)
//   - interprocess -> MQ (source.preparation only; consumed by cmd/sourcejob)
//
// Register in-process consumers via InProcess.Subscribe; register MQ consumers
// via InterProcess.Subscribe.
type CompositeEventBus struct {
	InProcess    InProcessEventBus
	InterProcess InterProcessEventBus
}

func NewCompositeEventBus(inprocess InProcessEventBus, interprocess InterProcessEventBus) *CompositeEventBus {
	if inprocess == nil {
		inprocess = NewInProcessEventBus()
	}
	return &CompositeEventBus{
		InProcess:    inprocess,
		InterProcess: interprocess,
	}
}

func (b *CompositeEventBus) Publish(ctx context.Context, evt event.Event) error {
	switch evt.Category() {
	case event.CategoryInProcess:
		return b.InProcess.Publish(ctx, evt)
	case event.CategoryInterProcess:
		return b.InterProcess.Publish(ctx, evt)
	default:
		return errors.Errorf("unknown event category: %s", evt.Category())
	}
}

func (b *CompositeEventBus) Close(ctx context.Context) error {
	return stderr.Join(b.InProcess.Close(ctx), b.InterProcess.Close(ctx))
}
