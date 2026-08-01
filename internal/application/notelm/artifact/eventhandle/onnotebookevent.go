package eventhandle

import (
	"context"
	"log/slog"

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
	ctx context.Context,
	bus eventbus.EventBus,
	handler *OnNotebookEventHandler,
) error {
	return eventbus.SubscribeNotebookEvent(ctx, bus, handler.Handle)
}
