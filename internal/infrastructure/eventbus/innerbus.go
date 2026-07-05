package eventbus

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type innerEventBus struct {
	mu   sync.RWMutex
	subs map[string][]InnerEventHandler
}

func NewInnerEventBus() EventBus {
	return &innerEventBus{
		subs: make(map[string][]InnerEventHandler),
	}
}

func (b *innerEventBus) Publish(ctx context.Context, evt event.Event) error {
	if evt.Category() != event.CategoryInner {
		return errors.New("event category is not inner")
	}

	b.mu.RLock()
	handlers := append([]InnerEventHandler(nil), b.subs[evt.Topic()]...)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "inner event handler failed",
				slog.String("topic", evt.Topic()),
				slog.Any("err", err),
			)
		}
	}
	return nil
}

func (b *innerEventBus) Subscribe(
	ctx context.Context,
	topic, _ string,
	handler EventBusMessageHandler,
) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	return b.subscribeInner(topic, func(ctx context.Context, evt event.Event) error {
		return handler(ctx, Envelope{
			Topic:   evt.Topic(),
			Key:     evt.Key(),
			Headers: evt.Headers(),
			Inner:   evt,
		})
	})
}

func (b *innerEventBus) subscribeInner(topic string, handler InnerEventHandler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], handler)
	b.mu.Unlock()
	return nil
}

func (b *innerEventBus) Close(ctx context.Context) error {
	b.mu.Lock()
	b.subs = make(map[string][]InnerEventHandler)
	b.mu.Unlock()
	return nil
}
