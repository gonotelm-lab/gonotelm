package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
)

type DeleteNotebookMemberHandler struct {
	*baseHandler
	memberRepo notebookrepo.MemberRepository
}

func NewDeleteNotebookMemberHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	memberRepo notebookrepo.MemberRepository,
	userRepo identrepo.UserRepository,
) *DeleteNotebookMemberHandler {
	return &DeleteNotebookMemberHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo),
		memberRepo:  memberRepo,
	}
}

func (h *DeleteNotebookMemberHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
	userId valobj.Uid,
) error {
	_, err := withNotebookAccess(h.baseHandler, ctx, notebookId, accessManageSharing,
		func(ctx context.Context, access *notebookentity.Access) (struct{}, error) {
			return h.deleteMember(ctx, access, userId)
		},
	)
	return err
}

func (h *DeleteNotebookMemberHandler) deleteMember(
	ctx context.Context,
	access *notebookentity.Access,
	userId valobj.Uid,
) (struct{}, error) {
	notebookId := access.Notebook.Id

	if _, err := h.memberRepo.FindByNotebookIdAndUserId(ctx, notebookId, userId); err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return struct{}{}, notebookerrors.ErrMemberNotFound
		}
		return struct{}{}, errors.WithMessage(err, "get notebook member failed")
	}

	if err := h.memberRepo.Delete(ctx, notebookId, userId); err != nil {
		return struct{}{}, errors.WithMessage(err, "delete notebook member failed")
	}

	return struct{}{}, nil
}
