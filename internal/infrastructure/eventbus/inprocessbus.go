package eventbus

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/exchange"
)

// in-process event communication
type inProcessEventBus struct {
	ex *exchange.Exchange[event.Event]
}

func NewInProcessEventBus() InProcessEventBus {
	return &inProcessEventBus{
		ex: exchange.New[event.Event](),
	}
}

func (b *inProcessEventBus) Publish(ctx context.Context, evt event.Event) error {
	if evt.Category() != event.CategoryInProcess {
		return errors.New("event category is not inprocess")
	}
	return b.ex.Publish(ctx, evt.Topic(), evt)
}

func (b *inProcessEventBus) Subscribe(topic string, handler InProcessEventHandler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}
	if _, err := b.ex.Subscribe(topic, func(ctx context.Context, _ exchange.Topic, evt event.Event) {
		if err := handler(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "inprocess event handler failed",
				slog.String("topic", evt.Topic()),
				slog.Any("err", err),
			)
		}
	}); err != nil {
		return err
	}
	return nil
}

func (b *inProcessEventBus) Close(ctx context.Context) error {
	return b.ex.Terminate(ctx)
}
