package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
)

type RenameFavouriteHandler struct {
	*baseHandler
	favouriteNotebookRepo repository.FavouriteNotebookRepository
}

func NewRenameFavouriteHandler(
	favouriteRepo repository.FavouriteRepository,
	favouriteNotebookRepo repository.FavouriteNotebookRepository,
) *RenameFavouriteHandler {
	return &RenameFavouriteHandler{
		baseHandler:           newBaseHandler(favouriteRepo),
		favouriteNotebookRepo: favouriteNotebookRepo,
	}
}

func (h *RenameFavouriteHandler) Handle(
	ctx context.Context,
	id valobj.Id,
	name string,
) (*FavouriteView, error) {
	f, err := h.loadOwnedFavourite(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := f.Rename(name); err != nil {
		return nil, err
	}
	if err := h.favouriteRepo.Save(ctx, f); err != nil {
		return nil, wrap(err, "save favourite failed")
	}
	count, err := h.favouriteNotebookRepo.CountByFavouriteId(ctx, f.Id)
	if err != nil {
		return nil, wrap(err, "count favourite notebooks failed")
	}
	return &FavouriteView{
		Id:            f.Id,
		Name:          f.Name,
		NotebookCount: count,
		CreatedAt:     f.CreateTime.Value(),
	}, nil
}
