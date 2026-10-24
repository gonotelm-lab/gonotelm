package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetNotebookHandler struct {
	*baseHandler
}

func NewGetNotebookHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
) *GetNotebookHandler {
	return &GetNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo),
	}
}

func (h *GetNotebookHandler) Handle(
	ctx context.Context,
	id valobj.Id,
) (*Notebook, error) {
	return withNotebookAccess(h.baseHandler, ctx, id, accessView, h.getNotebook)
}

func (h *GetNotebookHandler) getNotebook(
	ctx context.Context,
	access *notebookentity.Access,
) (*Notebook, error) {
	result := &Notebook{
		Notebook: access.Notebook,
	}
	if err := h.attachNotebookOwners(ctx, []*Notebook{result}); err != nil {
		return nil, pkgerr.WithMessagef(err, "attach notebook owners failed, notebook_id=%s", access.Notebook.Id)
	}

	return result, nil
}
