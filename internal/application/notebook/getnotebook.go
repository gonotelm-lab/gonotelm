package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetNotebookHandler struct {
	notebookRepo notebookrepo.Repository
}

func NewGetNotebookHandler(notebookRepo notebookrepo.Repository) *GetNotebookHandler {
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
