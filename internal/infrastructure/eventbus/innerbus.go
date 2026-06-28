package eventbus

import (
	"context"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type innerEventBus struct {
	mu   sync.RWMutex
	subs map[string][]EventBusMessageHandler
}

func NewInnerEventBus() EventBus {
	return &innerEventBus{
		subs: make(map[string][]EventBusMessageHandler),
	}
}

func (b *innerEventBus) Publish(ctx context.Context, evt event.Event) error {
	if evt.Category() != event.CategoryInner {
		return errors.New("event category is not inner")
	}

	val, err := sonic.Marshal(evt.Value())
	if err != nil {
		return errors.Wrap(err, "marshal event value failed")
	}

	env := Envelope{
		Topic:   evt.Topic(),
		Key:     evt.Key(),
		Value:   val,
		Headers: evt.Headers(),
	}

	b.mu.RLock()
	handlers := append([]EventBusMessageHandler(nil), b.subs[evt.Topic()]...)
	b.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, env); err != nil {
			return err
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

	b.mu.Lock()
	b.subs[topic] = append(b.subs[topic], handler)
	b.mu.Unlock()
	return nil
}

func (b *innerEventBus) Close(ctx context.Context) error {
	b.mu.Lock()
	b.subs = make(map[string][]EventBusMessageHandler)
	b.mu.Unlock()
	return nil
}
