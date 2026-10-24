package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
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
	_, err := withNotebookAccess(h.baseHandler, ctx, id, accessManageNotebook,
		func(ctx context.Context, access *notebookentity.Access) (struct{}, error) {
			return h.updateName(ctx, access, name)
		},
	)
	return err
}

func (h *UpdateNotebookNameHandler) updateName(
	ctx context.Context,
	access *notebookentity.Access,
	name string,
) (struct{}, error) {
	n := access.Notebook
	if err := n.UpdateName(name); err != nil {
		return struct{}{}, errors.WithMessage(err, "update notebook name failed")
	}

	if err := h.notebookRepo.Save(ctx, n); err != nil {
		return struct{}{}, errors.WithMessage(err, "update notebook name failed")
	}

	return struct{}{}, nil
}
