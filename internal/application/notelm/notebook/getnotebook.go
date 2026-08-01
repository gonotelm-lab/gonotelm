package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
)

type GetNotebookHandler struct {
	*baseHandler
}

func NewGetNotebookHandler(notebookRepo notebookrepo.Repository) *GetNotebookHandler {
	return &GetNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo),
	}
}

func (h *GetNotebookHandler) Handle(
	ctx context.Context,
	id valobj.Id,
) (*notebookentity.Notebook, error) {
	notebook, err := h.handle(ctx, id)
	if err != nil {
		return nil, err
	}

	return notebook, nil
}
