package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
)

type DeleteFavouriteHandler struct {
	*baseHandler
}

func NewDeleteFavouriteHandler(
	favouriteRepo repository.FavouriteRepository,
) *DeleteFavouriteHandler {
	return &DeleteFavouriteHandler{baseHandler: newBaseHandler(favouriteRepo)}
}

func (h *DeleteFavouriteHandler) Handle(ctx context.Context, id valobj.Id) error {
	f, err := h.loadOwnedFavourite(ctx, id)
	if err != nil {
		return err
	}
	f.Delete()
	if err := h.favouriteRepo.Save(ctx, f); err != nil {
		return wrap(err, "delete favourite failed")
	}
	return nil
}
