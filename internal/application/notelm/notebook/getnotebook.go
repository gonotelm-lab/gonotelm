package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
)

type GetNotebookHandler struct {
	*baseHandler
}

func NewGetNotebookHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *GetNotebookHandler {
	return &GetNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService),
	}
}

func (h *GetNotebookHandler) Handle(
	ctx context.Context,
	id valobj.Id,
) (*notebookentity.Notebook, error) {
	access, err := h.handle(ctx, id)
	if err != nil {
		return nil, err
	}

	return access.Notebook, nil
}
