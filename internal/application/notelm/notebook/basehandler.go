package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	idententity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type baseHandler struct {
	notebookRepo          notebookrepo.Repository
	notebookAccessService *notebookservice.AccessService
	userRepo              identrepo.UserRepository
}

func newBaseHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
) *baseHandler {
	return &baseHandler{
		notebookRepo:          notebookRepo,
		notebookAccessService: notebookAccessService,
		userRepo:              userRepo,
	}
}

// 无任何角色时返回 404，避免泄露私有 notebook 是否存在。检查用户是否有权限获取笔记
func (h *baseHandler) checkViewNotebookAccess(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.notebookAccessService.Resolve(ctx, notebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", notebookId)
	}

	if err := access.CanViewReturnErr(); err != nil {
		return nil, err // 统一返回NOT_FOUND
	}

	return access, nil
}

func (h *baseHandler) checkManageNotebookAccess(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.checkViewNotebookAccess(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	if !access.CanManageNotebook() {
		return nil, errors.ErrPermission.Msgf("notebook owner required, notebook_id=%s", notebookId)
	}

	return access, nil
}

func (h *baseHandler) checkManageSharingAccess(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.checkViewNotebookAccess(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	if !access.CanManageSharing() {
		return nil, errors.ErrPermission.Msgf("notebook sharing owner required, notebook_id=%s", notebookId)
	}

	return access, nil
}

func (h *baseHandler) attachNotebookOwners(ctx context.Context, notebooks []*Notebook) error {
	ownerIds := make([]valobj.Uid, 0, len(notebooks))
	for _, n := range notebooks {
		ownerIds = append(ownerIds, n.OwnerId)
	}
	ownerId := pkgslices.Unique(ownerIds)
	if len(ownerId) > 0 {
		users, err := h.userRepo.BatchGetById(ctx, ownerId)
		if err != nil {
			return errors.WithMessage(err, "user repo batch get failed")
		}

		userMap := pkgslices.AsMapF(users, func(t *idententity.User) valobj.Uid { return t.Id })
		// attach user
		for _, n := range notebooks {
			if owner, ok := userMap[n.OwnerId]; ok && owner != nil {
				n.Owner = owner
			}
		}
	}

	return nil
}
