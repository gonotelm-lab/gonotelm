package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
)

// Envelope is the transport boundary for inter-process (MQ) messages.
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

// Publisher publishes events; CompositeEventBus routes them by category.
type Publisher interface {
	Publish(ctx context.Context, evt event.Event) error
}

// InProcessEventBus dispatches events to in-process handlers without serialization.
type InProcessEventBus interface {
	Publisher
	Subscribe(topic string, handler InProcessEventHandler) error
	Close(ctx context.Context) error
}

// InterProcessEventBus sends events through the message queue (MQ).
type InterProcessEventBus interface {
	Publisher
	Subscribe(ctx context.Context, topic, groupID string, handler InterProcessEventHandler) error
	Close(ctx context.Context) error
}

// InProcessEventHandler receives in-process events as strongly-typed event.Event
// (use AssertEvent to unwrap concrete event types).
type InProcessEventHandler func(ctx context.Context, evt event.Event) error

// InterProcessEventHandler receives inter-process messages as Envelope.
type InterProcessEventHandler func(ctx context.Context, env Envelope) error
