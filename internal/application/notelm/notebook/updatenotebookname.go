package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateNotebookNameHandler struct {
	*baseHandler
}

func NewUpdateNotebookNameHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *UpdateNotebookNameHandler {
	return &UpdateNotebookNameHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService),
	}
}

func (h *UpdateNotebookNameHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	name string,
) error {
	access, err := h.handleOwner(ctx, id)
	if err != nil {
		return err
	}

	n := access.Notebook
	err = n.UpdateName(name)
	if err != nil {
		return errors.WithMessage(err, "update notebook name failed")
	}

	err = h.notebookRepo.Save(ctx, n)
	if err != nil {
		return errors.WithMessage(err, "update notebook name failed")
	}

	return nil
}
