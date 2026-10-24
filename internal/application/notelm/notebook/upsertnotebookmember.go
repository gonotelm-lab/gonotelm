package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook/vo"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UpsertNotebookMemberCommand struct {
	Email string
	Role  string
}

type UpsertNotebookMemberHandler struct {
	*baseHandler
	memberRepo notebookrepo.MemberRepository
	userRepo   identityrepo.UserRepository
}

func NewUpsertNotebookMemberHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	memberRepo notebookrepo.MemberRepository,
	userRepo identityrepo.UserRepository,
) *UpsertNotebookMemberHandler {
	return &UpsertNotebookMemberHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo),
		memberRepo:  memberRepo,
		userRepo:    userRepo,
	}
}

func (h *UpsertNotebookMemberHandler) Handle(
	ctx context.Context,
	notebookId valobj.Id,
	cmd *UpsertNotebookMemberCommand,
) (*notebookentity.NotebookMember, error) {
	return withNotebookAccess(h.baseHandler, ctx, notebookId, accessManageSharing,
		func(ctx context.Context, access *notebookentity.Access) (*notebookentity.NotebookMember, error) {
			return h.upsertMember(ctx, access, cmd)
		},
	)
}

func (h *UpsertNotebookMemberHandler) upsertMember(
	ctx context.Context,
	access *notebookentity.Access,
	cmd *UpsertNotebookMemberCommand,
) (*notebookentity.NotebookMember, error) {
	notebookId := access.Notebook.Id

	role := vo.Role(cmd.Role)
	if !role.IsMemberRole() {
		return nil, notebookerrors.ErrInvalidMemberRole
	}

	email := identityentity.NormalizeEmail(cmd.Email)
	user, err := h.userRepo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, identityerr.ErrUserNotFound) {
			return nil, notebookerrors.ErrShareUserNotFound
		}
		return nil, errors.WithMessage(err, "get share user failed")
	}

	if user.IsBanned() {
		return nil, notebookerrors.ErrShareUserBanned
	}

	if !access.CanAcceptAsMember(user.Id) {
		return nil, notebookerrors.ErrCannotShareWithOwner
	}

	member, err := h.memberRepo.FindByNotebookIdAndUserId(ctx, notebookId, user.Id)
	if err == nil {
		if err := member.ChangeRole(role); err != nil {
			return nil, err
		}
	} else {
		if !errors.Is(err, errors.ErrNoRecord) {
			return nil, errors.WithMessage(err, "get notebook member failed")
		}

		member, err = notebookentity.NewNotebookMember(notebookId, user.Id, role)
		if err != nil {
			return nil, err
		}
	}

	if err := h.memberRepo.Save(ctx, member); err != nil {
		return nil, errors.WithMessage(err, "save notebook member failed")
	}

	return member, nil
}
