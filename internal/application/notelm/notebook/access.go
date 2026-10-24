package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetNotebookAccessHandler struct {
	notebookAccessService *notebookservice.AccessService
}

func NewGetNotebookAccessHandler(
	notebookAccessService *notebookservice.AccessService,
) *GetNotebookAccessHandler {
	return &GetNotebookAccessHandler{notebookAccessService: notebookAccessService}
}

func (h *GetNotebookAccessHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.notebookAccessService.Resolve(ctx, notebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook access failed, notebook_id=%s", notebookId)
	}
	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	return access, nil
}
