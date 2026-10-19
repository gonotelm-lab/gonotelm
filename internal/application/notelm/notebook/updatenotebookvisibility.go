package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
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
	access, err := h.checkManageSharingAccess(ctx, notebookId)
	if err != nil {
		return err
	}

	if err := access.Notebook.SetVisibility(visibility); err != nil {
		return err
	}

	if err := h.notebookRepo.Save(ctx, access.Notebook); err != nil {
		return errors.WithMessage(err, "save notebook visibility failed")
	}

	return nil
}
