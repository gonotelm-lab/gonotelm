package eventbus

import (
	"context"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/mq"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type outerEventBus struct {
	producer  mq.Producer
	mqFactory *mq.MQ

	mu        sync.Mutex
	consumers []mq.Consumer
}

func NewOuterEventBus(mqFactory *mq.MQ) EventBus {
	evbus := &outerEventBus{
		mqFactory: mqFactory,
	}
	evbus.producer = evbus.mqFactory.NewProducer()
	return evbus
}

func (b *outerEventBus) Publish(ctx context.Context, evt event.Event) error {
	if evt.Category() != event.CategoryOuter {
		return errors.New("event category is not outer")
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

func (b *outerEventBus) Subscribe(
	ctx context.Context,
	topic, groupID string,
	handler EventBusMessageHandler,
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

func (b *outerEventBus) Close(ctx context.Context) error {
	b.mu.Lock()
	consumers := b.consumers
	b.consumers = nil
	b.mu.Unlock()

	var closeErr error
	for _, consumer := range consumers {
		if err := consumer.Close(ctx); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	if b.producer != nil {
		if err := b.producer.Close(ctx); err != nil && closeErr == nil {
			closeErr = err
		}
	}

	return closeErr
}
