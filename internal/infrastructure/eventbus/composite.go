package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// CompositeEventBus routes publish by event category:
//   - inner -> in-process handlers (notebook.deleted, source.index, source.deleted, ...)
//   - outer -> MQ (source.preparation only)
//
// Subscribe registers outer (MQ) consumers; use SubscribeInner for in-process consumers.
type CompositeEventBus struct {
	Inner EventBus
	outer EventBus
}

func NewCompositeEventBus(inner, outer EventBus) *CompositeEventBus {
	if inner == nil {
		inner = NewInnerEventBus()
	}
	return &CompositeEventBus{
		Inner: inner,
		outer: outer,
	}
}

func (b *CompositeEventBus) Publish(ctx context.Context, evt event.Event) error {
	switch evt.Category() {
	case event.CategoryInner:
		return b.Inner.Publish(ctx, evt)
	case event.CategoryOuter:
		return b.outer.Publish(ctx, evt)
	default:
		return errors.Errorf("unknown event category: %s", evt.Category())
	}
}

// Subscribe registers an outer (MQ) consumer. Only source.preparation uses this path today.
func (b *CompositeEventBus) Subscribe(
	ctx context.Context,
	topic, groupID string,
	handler EventBusMessageHandler,
) error {
	return b.outer.Subscribe(ctx, topic, groupID, handler)
}

// SubscribeInner registers an in-process consumer on the inner bus.
func (b *CompositeEventBus) SubscribeInner(
	ctx context.Context,
	topic string,
	handler InnerEventHandler,
) error {
	inner, ok := b.Inner.(*innerEventBus)
	if !ok {
		return errors.New("inner event bus does not support direct subscription")
	}
	return inner.subscribeInner(topic, handler)
}

func (b *CompositeEventBus) Close(ctx context.Context) error {
	var closeErr error
	if err := b.Inner.Close(ctx); err != nil {
		closeErr = err
	}
	if err := b.outer.Close(ctx); err != nil && closeErr == nil {
		closeErr = err
	}
	return closeErr
}

// AsComposite returns the composite bus when bus is one; used at composition roots.
func AsComposite(bus EventBus) (*CompositeEventBus, error) {
	composite, ok := bus.(*CompositeEventBus)
	if !ok {
		return nil, errors.New("event bus is not composite")
	}
	return composite, nil
}
