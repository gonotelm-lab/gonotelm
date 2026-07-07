package eventhandle

import (
	"context"
	"log/slog"

	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteNotebookArtifactTasksHandler struct {
	artifactTaskRepo *repository.ArtifactTaskRepository
}

func NewDeleteNotebookArtifactTasksHandler(
	artifactTaskRepo *repository.ArtifactTaskRepository,
) *DeleteNotebookArtifactTasksHandler {
	return &DeleteNotebookArtifactTasksHandler{
		artifactTaskRepo: artifactTaskRepo,
	}
}

func (h *DeleteNotebookArtifactTasksHandler) Handle(
	ctx context.Context,
	evt *notebookevent.Event,
) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	if err := h.artifactTaskRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		return errors.WithMessagef(err, "delete artifact tasks failed, notebook_id=%s", notebookId)
	}

	slog.InfoContext(ctx, "cleaned up artifact tasks for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
	)

	return nil
}

func RegisterNotebookDeletedConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *DeleteNotebookArtifactTasksHandler,
) error {
	return eventbus.SubscribeNotebookDeleted(ctx, bus, handler.Handle)
}
