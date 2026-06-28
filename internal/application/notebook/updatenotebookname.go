package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateNotebookNameHandler struct {
	notebookRepo notebook.Repository
}

func NewUpdateNotebookNameHandler(notebookRepo notebook.Repository) *UpdateNotebookNameHandler {
	return &UpdateNotebookNameHandler{
		notebookRepo: notebookRepo,
	}
}

func (h *UpdateNotebookNameHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	name string,
) error {
	n, err := h.notebookRepo.FindById(ctx, id)
	if err != nil {
		return errors.WithMessage(err, "get notebook failed")
	}

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
