package notebook

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// DeleteCoverHandler 清除生效封面与在途 key。
type DeleteCoverHandler struct {
	*baseHandler
	distLock adapter.DistributedLock
}

func NewDeleteCoverHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	userRepo identrepo.UserRepository,
	objectStore adapter.ObjectStore,
	distLock adapter.DistributedLock,
) *DeleteCoverHandler {
	h := &DeleteCoverHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService, userRepo, objectStore).withObjectStore(objectStore),
		distLock:    distLock,
	}

	return h
}

type DeleteCoverCommand struct {
	NotebookId valobj.Id
}

type deleteCoverResult struct{}

func (h *DeleteCoverHandler) Handle(ctx context.Context, cmd *DeleteCoverCommand) error {
	_, err := withNotebookAccess(h.baseHandler, ctx, cmd.NotebookId, accessManageNotebook,
		func(ctx context.Context, access *notebookentity.Access) (*deleteCoverResult, error) {
			return h.deleteCoverWithLock(ctx, access.Notebook)
		})
	if err != nil {
		return errors.WithMessagef(err, "delete notebook cover failed, notebook_id=%s", cmd.NotebookId)
	}

	return nil
}

func (h *DeleteCoverHandler) deleteCoverWithLock(
	ctx context.Context,
	n *notebookentity.Notebook,
) (*deleteCoverResult, error) {
	var result *deleteCoverResult
	err := h.distLock.WithLock(ctx, notebookCoverLockKey(n.Id), func(ctx context.Context) error {
		var err error
		result, err = h.deleteCover(ctx, n)
		return err
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "with cover lock failed, notebook_id=%s", n.Id)
	}

	return result, nil
}

func (h *DeleteCoverHandler) deleteCover(
	ctx context.Context,
	n *notebookentity.Notebook,
) (*deleteCoverResult, error) {
	if !n.Cover.Valid() && !n.PendingCover.Valid() {
		// 全空视为已删，幂等返回
		return &deleteCoverResult{}, nil
	}

	h.deleteCoverObject(ctx, n.Id, n.Cover)
	h.deleteCoverObject(ctx, n.Id, n.PendingCover)

	n.ClearCover()
	if err := h.notebookRepo.Save(ctx, n); err != nil {
		return nil, errors.WithMessage(err, "save notebook cover failed")
	}

	return &deleteCoverResult{}, nil
}
