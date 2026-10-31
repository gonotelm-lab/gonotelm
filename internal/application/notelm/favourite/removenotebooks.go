package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
)

type RemoveNotebooksHandler struct {
	*baseHandler
	favouriteNotebookRepo repository.FavouriteNotebookRepository
}

func NewRemoveNotebooksHandler(
	favouriteRepo repository.FavouriteRepository,
	favouriteNotebookRepo repository.FavouriteNotebookRepository,
) *RemoveNotebooksHandler {
	return &RemoveNotebooksHandler{
		baseHandler:           newBaseHandler(favouriteRepo),
		favouriteNotebookRepo: favouriteNotebookRepo,
	}
}

// 移除收藏夹中某个收藏的笔记本
func (h *RemoveNotebooksHandler) Handle(
	ctx context.Context,
	favouriteId valobj.Id, // 收藏夹id
	notebookIds []valobj.Id, // 笔记本id
) error {
	if _, err := h.loadOwnedFavourite(ctx, favouriteId); err != nil {
		return err
	}

	ids := pkgslices.Unique(notebookIds)
	if len(ids) == 0 {
		return errors.ErrParams.Msg("notebook_ids is required")
	}
	if len(ids) > entity.MaxRemoveNotebookIdsPerReq {
		return errors.ErrParams.Msgf("notebook_ids exceeds max %d", entity.MaxRemoveNotebookIdsPerReq)
	}

	if err := h.favouriteNotebookRepo.Delete(ctx, favouriteId, ids); err != nil {
		return errors.WithMessage(err, "delete favourite notebooks failed")
	}

	return nil
}
