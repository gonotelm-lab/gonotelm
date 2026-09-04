package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteNotebookHandler struct {
	*baseHandler
	eventBus eventbus.Publisher
}

func NewDeleteNotebookHandler(notebookRepo notebookrepo.Repository, eventBus eventbus.Publisher) *DeleteNotebookHandler {
	return &DeleteNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo),
		eventBus:    eventBus,
	}
}

func (h *DeleteNotebookHandler) Handle(ctx context.Context, id valobj.Id) error {
	n, err := h.handle(ctx, id)
	if err != nil {
		if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
			return nil
		}
		return err
	}

	n.Delete()
	err = h.notebookRepo.Save(ctx, n)
	if err != nil {
		return errors.WithMessage(err, "delete notebook failed")
	}

	for _, event := range n.PullEvents() {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			slog.ErrorContext(ctx, "publish notebook deleted event failed", "error", err, "notebook_id", id)
		}
	}

	return nil
}
