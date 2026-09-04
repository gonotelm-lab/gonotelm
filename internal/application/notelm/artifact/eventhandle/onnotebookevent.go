package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type OnNotebookEventHandler struct {
	artifactRepo artifactrepo.Repository
}

func NewOnNotebookEventHandler(
	artifactRepo artifactrepo.Repository,
) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{
		artifactRepo: artifactRepo,
	}
}

func (h *OnNotebookEventHandler) Handle(
	ctx context.Context,
	evt *notebookevent.Event,
) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	if err := h.artifactRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		return errors.WithMessagef(err, "delete artifact tasks failed, notebook_id=%s", notebookId)
	}

	slog.InfoContext(ctx, "cleaned up artifact tasks for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
	)

	return nil
}

func RegisterNotebookEventConsumer(
	bus eventbus.InProcessEventBus,
	handler *OnNotebookEventHandler,
) error {
	return bus.Subscribe(notebookevent.TopicNotebookEvent,
		func(ctx context.Context, evt event.Event) error {
			nbEvt, err := eventbus.AssertEvent[*notebookevent.Event](evt)
			if err != nil {
				return err
			}
			return handler.Handle(ctx, nbEvt)
		},
	)
}
