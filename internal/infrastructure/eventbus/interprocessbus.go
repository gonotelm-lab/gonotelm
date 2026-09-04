package eventbus

import (
	"context"
	stderr "errors"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type interProcessEventBus struct {
	producer  mq.Producer
	mqFactory *mq.MessageQueue

	mu        sync.Mutex
	consumers []mq.Consumer
}

func NewInterProcessEventBus(mqFactory *mq.MessageQueue) InterProcessEventBus {
	bus := &interProcessEventBus{
		mqFactory: mqFactory,
	}
	bus.producer = bus.mqFactory.NewProducer()
	return bus
}

func (b *interProcessEventBus) Publish(ctx context.Context, evt event.Event) error {
	if evt.Category() != event.CategoryInterProcess {
		return errors.New("event category is not interprocess")
	}

	val, err := sonic.Marshal(evt.Value())
	if err != nil {
		return errors.Wrap(err, "marshal event value failed")
	}

	hds := make([]mq.MessageHeader, 0, len(evt.Headers()))
	for _, h := range evt.Headers() {
		hds = append(hds, mq.MessageHeader{
			Key:   h.Key,
			Value: h.Value,
		})
	}

	return b.producer.Send(ctx, &mq.ProducerSendRequest{
		Topic:   evt.Topic(),
		Key:     []byte(evt.Key()),
		Value:   val,
		Headers: hds,
	})
}

func (b *interProcessEventBus) Subscribe(
	ctx context.Context,
	topic, groupID string,
	handler InterProcessEventHandler,
) error {
	if handler == nil {
		return errors.New("handler is nil")
	}

	consumer := b.mqFactory.NewConsumer(topic, groupID)

	b.mu.Lock()
	b.consumers = append(b.consumers, consumer)
	b.mu.Unlock()

	return consumer.Subscribe(ctx, topic, func(ctx context.Context, msg mq.Message) error {
		hds := make([]event.Header, 0, len(msg.Headers()))
		for _, h := range msg.Headers() {
			hds = append(hds, event.Header{
				Key:   h.Key,
				Value: h.Value,
			})
		}

		return handler(ctx, Envelope{
			Topic:   msg.Topic(),
			Key:     string(msg.Key()),
			Value:   msg.Value(),
			Headers: hds,
		})
	})
}

func (b *interProcessEventBus) Close(ctx context.Context) error {
	b.mu.Lock()
	consumers := b.consumers
	b.consumers = nil
	b.mu.Unlock()

	var closeErr error
	for _, consumer := range consumers {
		if err := consumer.Close(ctx); err != nil {
			closeErr = stderr.Join(closeErr, err)
		}
	}
	if b.producer != nil {
		if err := b.producer.Close(ctx); err != nil {
			closeErr = stderr.Join(closeErr, err)
		}
	}

	return closeErr
}
