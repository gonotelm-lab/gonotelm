package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
)

func SubscribeNotebookDeleted(
	ctx context.Context,
	bus EventBus,
	handler func(ctx context.Context, evt *notebookevent.Event) error,
) error {
	composite, err := AsComposite(bus)
	if err != nil {
		return err
	}

	return composite.SubscribeInner(ctx, notebookevent.TopicNotebookEvent,
		func(ctx context.Context, evt event.Event) error {
			nbEvt, err := AssertEvent[*notebookevent.Event](evt)
			if err != nil {
				return err
			}
			return handler(ctx, nbEvt)
		},
	)
}
