package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateNotebookVisibilityHandler struct {
	*baseHandler
}

func NewUpdateNotebookVisibilityHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
) *UpdateNotebookVisibilityHandler {
	return &UpdateNotebookVisibilityHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo),
	}
}

func (h *UpdateNotebookVisibilityHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
	visibility vo.Visibility,
) error {
	_, err := withNotebookAccess(h.baseHandler, ctx, notebookId, accessManageSharing,
		func(ctx context.Context, access *notebookentity.Access) (struct{}, error) {
			return h.updateVisibility(ctx, access, visibility)
		},
	)
	return err
}

func (h *UpdateNotebookVisibilityHandler) updateVisibility(
	ctx context.Context,
	access *notebookentity.Access,
	visibility vo.Visibility,
) (struct{}, error) {
	if err := access.Notebook.SetVisibility(visibility); err != nil {
		return struct{}{}, err
	}

	if err := h.notebookRepo.Save(ctx, access.Notebook); err != nil {
		return struct{}{}, errors.WithMessage(err, "save notebook visibility failed")
	}

	return struct{}{}, nil
}
