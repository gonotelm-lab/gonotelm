package eventbus

import (
	"context"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type inProcessEventBus struct {
	mu   sync.RWMutex
	subs map[string][]InProcessEventHandler
}

func NewInProcessEventBus() InProcessEventBus {
	return &inProcessEventBus{
		subs: make(map[string][]InProcessEventHandler),
	}
}

func (b *inProcessEventBus) Publish(ctx context.Context, evt event.Event) error {
	if evt.Category() != event.CategoryInProcess {
		return errors.New("event category is not inprocess")
	}

	b.mu.RLock()
	handlers := append([]InProcessEventHandler(nil), b.subs[evt.Topic()]...)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "inprocess event handler failed",
				slog.String("topic", evt.Topic()),
				slog.Any("err", err),
			)
		}
	}
	return nil
}

func (b *inProcessEventBus) Subscribe(topic string, handler InProcessEventHandler) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], handler)
	b.mu.Unlock()
	return nil
}

func (b *inProcessEventBus) Close(ctx context.Context) error {
	b.mu.Lock()
	b.subs = make(map[string][]InProcessEventHandler)
	b.mu.Unlock()
	return nil
}
