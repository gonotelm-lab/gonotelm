package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type CreateNotebookHandler struct {
	notebookRepo notebookrepo.Repository
	eventBus   eventbus.EventBus
}

func NewCreateNotebookHandler(notebookRepo notebookrepo.Repository, eventBus eventbus.EventBus) *CreateNotebookHandler {
	return &CreateNotebookHandler{
		notebookRepo: notebookRepo,
		eventBus:   eventBus,
	}
}

type CreateNotebookHandleCommand struct {
	Name    string
	Desc    string
	OwnerId string
}

func (h *CreateNotebookHandler) Handle(
	ctx context.Context,
	cmd *CreateNotebookHandleCommand,
) (valobj.Id, error) {
	notebook, err := notebook.NewNotebook(cmd.Name, cmd.Desc, cmd.OwnerId)
	if err != nil {
		return valobj.Id{}, errors.WithMessage(err, "create notebook failed")
	}

	err = h.notebookRepo.Save(ctx, notebook)
	if err != nil {
		return valobj.Id{}, errors.WithMessage(err, "save notebook failed")
	}

	for _, event := range notebook.PullEvents() {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			slog.ErrorContext(ctx, "publish notebook created event failed", "error", err, "notebook_id", notebook.Id)
		}
	}

	return notebook.Id, nil
}
