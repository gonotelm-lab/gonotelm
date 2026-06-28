package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
)

// Envelope is the transport boundary for subscribed messages.
// Infrastructure only carries raw bytes; application layer owns deserialization.
type Envelope struct {
	Topic   string
	Key     string
	Value   []byte
	Headers []event.Header
}

func (e Envelope) Header(key string) ([]byte, bool) {
	for _, h := range e.Headers {
		if h.Key == key {
			return h.Value, true
		}
	}
	return nil, false
}

type EventBusMessageHandler func(ctx context.Context, env Envelope) error

type EventBus interface {
	Publish(ctx context.Context, evt event.Event) error
	Subscribe(ctx context.Context, topic, groupID string, handler EventBusMessageHandler) error
	Close(ctx context.Context) error
}
