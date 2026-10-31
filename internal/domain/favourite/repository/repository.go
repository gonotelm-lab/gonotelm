package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/favourite/entity"
)

type Repository interface {
	// 删除收藏夹的同时级联删除收藏夹的笔记本记录
	Save(ctx context.Context, f *entity.Favourite) error
	Find(ctx context.Context, id valobj.Id) (*entity.Favourite, error)
	ListByUserId(ctx context.Context, userId valobj.Uid, spec *ListSpec) ([]*entity.Favourite, error)
	// ListByUserAndNotebook 返回该用户把 notebook 放进了哪些收藏夹（按 favourited_at 降序；无则空切片）。
	ListByUserAndNotebook(ctx context.Context, userId valobj.Uid, notebookId valobj.Id) ([]*entity.Favourite, error)
	CountByUserId(ctx context.Context, userId valobj.Uid) (int64, error)
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}

type NotebookRepository interface {
	Save(ctx context.Context, row *entity.FavouriteNotebook) error
	Find(ctx context.Context, favouriteId, notebookId valobj.Id) (*entity.FavouriteNotebook, error)
	ListByFavouriteId(ctx context.Context, favouriteId valobj.Id, spec *ListSpec) ([]*entity.FavouriteNotebook, error)
	CountByFavouriteId(ctx context.Context, favouriteId valobj.Id) (int64, error)
	Delete(ctx context.Context, favouriteId valobj.Id, notebookIds []valobj.Id) error
	DeleteByFavouriteId(ctx context.Context, favouriteId valobj.Id) error
	DeleteByUserId(ctx context.Context, userId valobj.Uid) error
}
