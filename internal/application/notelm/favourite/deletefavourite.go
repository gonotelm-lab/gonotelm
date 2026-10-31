package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteFavouriteHandler struct {
	*baseHandler
}

func NewDeleteFavouriteHandler(
	favouriteRepo repository.FavouriteRepository,
) *DeleteFavouriteHandler {
	return &DeleteFavouriteHandler{baseHandler: newBaseHandler(favouriteRepo)}
}

// 删除收藏夹
func (h *DeleteFavouriteHandler) Handle(ctx context.Context, id valobj.Id) error {
	f, err := h.loadOwnedFavourite(ctx, id)
	if err != nil {
		return err
	}
	f.Delete() // 删除收藏夹的同时级联删除底下的笔记本记录
	if err := h.favouriteRepo.Save(ctx, f); err != nil {
		return errors.WithMessage(err, "delete favourite failed")
	}

	return nil
}
