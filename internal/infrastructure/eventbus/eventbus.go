package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
)

// Envelope is the transport boundary for subscribed messages.
// Outer (MQ) consumers read Value bytes; inner consumers use Inner directly.
type Envelope struct {
	Topic   string
	Key     string
	Value   []byte
	Headers []event.Header

	Inner event.Event // set by inner bus only; no serialization
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

// InnerEventHandler receives in-process events without serialization.
type InnerEventHandler func(ctx context.Context, evt event.Event) error

type EventBus interface {
	Publish(ctx context.Context, evt event.Event) error
	Subscribe(ctx context.Context, topic, groupID string, handler EventBusMessageHandler) error
	Close(ctx context.Context) error
}
