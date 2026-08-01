package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	chatevent "github.com/gonotelm-lab/gonotelm/internal/domain/chat/event"
)

func SubscribeStreamTaskEvent(
	ctx context.Context,
	bus EventBus,
	handler func(ctx context.Context, evt *chatevent.StreamTaskEvent) error,
) error {
	composite, err := AsComposite(bus)
	if err != nil {
		return err
	}

	return composite.SubscribeInner(ctx, chatevent.StreamTaskTopic,
		func(ctx context.Context, evt event.Event) error {
			streamTaskEvt, err := AssertEvent[*chatevent.StreamTaskEvent](evt)
			if err != nil {
				return err
			}

			return handler(ctx, streamTaskEvt)
		},
	)
}
