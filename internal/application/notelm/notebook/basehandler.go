package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	notebookRepo          notebookrepo.Repository
	notebookAccessService *notebookservice.AccessService
}

func newBaseHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
) *baseHandler {
	return &baseHandler{
		notebookRepo:          notebookRepo,
		notebookAccessService: notebookAccessService,
	}
}

// 无任何角色时返回 404，避免泄露私有 notebook 是否存在。
func (h *baseHandler) handle(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.notebookAccessService.Resolve(ctx, notebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", notebookId)
	}

	if !access.CanView() {
		return nil, errors.ErrNoRecord.Msgf("notebook not found, notebook_id=%s", notebookId)
	}

	return access, nil
}

func (h *baseHandler) handleOwner(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.handle(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	if !access.CanManageNotebook() {
		return nil, errors.ErrPermission.Msgf("notebook owner required, notebook_id=%s", notebookId)
	}

	return access, nil
}

func (h *baseHandler) handleSharing(
	ctx context.Context,
	notebookId valobj.Id,
) (*notebookentity.Access, error) {
	access, err := h.handle(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	if !access.CanManageSharing() {
		return nil, errors.ErrPermission.Msgf("notebook sharing owner required, notebook_id=%s", notebookId)
	}

	return access, nil
}
