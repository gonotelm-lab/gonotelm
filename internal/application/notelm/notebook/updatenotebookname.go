package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateNotebookNameHandler struct {
	*baseHandler
}

func NewUpdateNotebookNameHandler(notebookRepo notebookrepo.Repository) *UpdateNotebookNameHandler {
	return &UpdateNotebookNameHandler{
		baseHandler: newBaseHandler(notebookRepo),
	}
}

func (h *UpdateNotebookNameHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	name string,
) error {
	n, err := h.handle(ctx, id)
	if err != nil {
		return err
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
