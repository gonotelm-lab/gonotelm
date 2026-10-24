package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/storekey"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
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

type accessKind int

const (
	accessView accessKind = iota
	accessManageNotebook
	accessManageSharing
)

type baseHandler struct {
	notebookRepo          notebookrepo.Repository
	notebookAccessService *notebookservice.AccessService
	userRepo              identrepo.UserRepository
	storeKeyResolver      *storekey.Resolver
}

func newBaseHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectPublicURLer,
) *baseHandler {
	return &baseHandler{
		notebookRepo:          notebookRepo,
		notebookAccessService: notebookAccessService,
		userRepo:              userRepo,
		storeKeyResolver:      storekey.New(objectStore),
	}
}

// withNotebookAccess 鉴权通过后把 access 注入业务方法
func withNotebookAccess[T any](
	h *baseHandler,
	ctx context.Context,
	notebookId valobj.Id,
	kind accessKind,
	fn func(ctx context.Context, access *notebookentity.Access) (T, error),
) (T, error) {
	access, err := h.resolveAccess(ctx, notebookId, kind)
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(ctx, access)
}

func (h *baseHandler) resolveAccess(
	ctx context.Context,
	notebookId valobj.Id,
	kind accessKind,
) (*notebookentity.Access, error) {
	access, err := h.notebookAccessService.Resolve(ctx, notebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", notebookId)
	}

	// 无任何角色时返回 404，避免泄露私有 notebook 是否存在
	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	switch kind {
	case accessView:
		return access, nil
	case accessManageNotebook:
		if !access.CanManageNotebook() {
			return nil, errors.ErrPermission.Msgf("notebook owner required, notebook_id=%s", notebookId)
		}
		return access, nil
	case accessManageSharing:
		if !access.CanManageSharing() {
			return nil, errors.ErrPermission.Msgf("notebook sharing owner required, notebook_id=%s", notebookId)
		}
		return access, nil
	default:
		return nil, errors.ErrInner.Msgf("unknown notebook access kind=%d", kind)
	}
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
		for _, n := range notebooks {
			if owner, ok := userMap[n.OwnerId]; ok && owner != nil {
				n.Owner = &NotebookOwner{
					Id:        owner.Id,
					Nickname:  owner.Nickname,
					AvatarURL: h.storeKeyResolver.ResolvePublic(ctx, owner.Avatar),
				}
			}
		}
	}

	return nil
}
