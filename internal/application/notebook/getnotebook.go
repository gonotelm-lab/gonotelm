package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetNotebookHandler struct {
	notebookRepo notebook.Repository
}

func NewGetNotebookHandler(notebookRepo notebook.Repository) *GetNotebookHandler {
	return &GetNotebookHandler{
		notebookRepo: notebookRepo,
	}
}

func (h *GetNotebookHandler) Handle(
	ctx context.Context,
	id valobj.Id,
) (*notebook.Notebook, error) {
	notebook, err := h.notebookRepo.FindById(ctx, id)
	if err != nil {
		return nil, errors.WithMessage(err, "get notebook failed")
	}

	return notebook, nil
}
