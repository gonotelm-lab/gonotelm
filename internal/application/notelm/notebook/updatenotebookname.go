package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpdateNotebookNameHandler struct {
	*baseHandler
}

func NewUpdateNotebookNameHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
) *UpdateNotebookNameHandler {
	return &UpdateNotebookNameHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo),
	}
}

func (h *UpdateNotebookNameHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	name string,
) error {
	access, err := h.checkManageNotebookAccess(ctx, id)
	if err != nil {
		return err
	}

	n := access.Notebook
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
