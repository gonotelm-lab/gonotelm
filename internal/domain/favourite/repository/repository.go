package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
)

type FavouriteRepository interface {
	Save(ctx context.Context, f *entity.Favourite) error
	Find(ctx context.Context, id valobj.Id) (*entity.Favourite, error)
	ListByUserId(ctx context.Context, userId valobj.Uid, spec *ListSpec) ([]*entity.Favourite, error)
	CountByUserId(ctx context.Context, userId valobj.Uid) (int64, error)
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}

type FavouriteNotebookRepository interface {
	Save(ctx context.Context, row *entity.FavouriteNotebook) error
	Find(ctx context.Context, favouriteId, notebookId valobj.Id) (*entity.FavouriteNotebook, error)
	ListByFavouriteId(ctx context.Context, favouriteId valobj.Id, spec *ListSpec) ([]*entity.FavouriteNotebook, error)
	// ListFavouritesByUserAndNotebook 返回该用户把 notebook 放进了哪些收藏夹（仅 Id/Name；无则空切片）。
	ListFavouritesByUserAndNotebook(ctx context.Context, userId valobj.Uid, notebookId valobj.Id) ([]*entity.Favourite, error)
	CountByFavouriteId(ctx context.Context, favouriteId valobj.Id) (int64, error)
	Delete(ctx context.Context, favouriteId valobj.Id, notebookIds []valobj.Id) error
	DeleteByFavouriteId(ctx context.Context, favouriteId valobj.Id) error
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}
