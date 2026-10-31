package favourite

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type ListFavouritesHandler struct {
	*baseHandler
	favouriteNotebookRepo repository.FavouriteNotebookRepository
}

func NewListFavouritesHandler(
	favouriteRepo repository.FavouriteRepository,
	favouriteNotebookRepo repository.FavouriteNotebookRepository,
) *ListFavouritesHandler {
	return &ListFavouritesHandler{
		baseHandler:           newBaseHandler(favouriteRepo),
		favouriteNotebookRepo: favouriteNotebookRepo,
	}
}

type ListFavouritesResult struct {
	Favourites []*FavouriteView
	Limit      int
	Offset     int
	HasMore    bool
}

func (h *ListFavouritesHandler) Handle(
	ctx context.Context,
	limit, offset int,
) (*ListFavouritesResult, error) {
	actor := pkgcontext.GetUserId(ctx)
	spec := &repository.ListSpec{Limit: limit + 1, Offset: offset}
	rows, err := h.favouriteRepo.ListByUserId(ctx, actor, spec)
	if err != nil {
		return nil, wrap(err, "list favourites failed")
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	views := make([]*FavouriteView, 0, len(rows))
	for _, f := range rows {
		count, err := h.favouriteNotebookRepo.CountByFavouriteId(ctx, f.Id)
		if err != nil {
			return nil, wrap(err, "count favourite notebooks failed")
		}
		views = append(views, &FavouriteView{
			Id:            f.Id,
			Name:          f.Name,
			NotebookCount: count,
			CreatedAt:     f.CreateTime.Value(),
		})
	}

	return &ListFavouritesResult{
		Favourites: views,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
	}, nil
}
