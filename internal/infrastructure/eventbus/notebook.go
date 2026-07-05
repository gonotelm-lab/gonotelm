package eventbus

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	notebookdomain "github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
)

func SubscribeNotebookDeleted(
	ctx context.Context,
	bus EventBus,
	handler func(ctx context.Context, evt *notebookdomain.Event) error,
) error {
	composite, err := AsComposite(bus)
	if err != nil {
		return err
	}

	return composite.SubscribeInner(ctx, notebookdomain.TopicNotebookEvent,
		func(ctx context.Context, evt event.Event) error {
			nbEvt, err := AssertEvent[*notebookdomain.Event](evt)
			if err != nil {
				return err
			}
			return handler(ctx, nbEvt)
		},
	)
}
