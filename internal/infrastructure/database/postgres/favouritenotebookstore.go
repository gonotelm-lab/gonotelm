package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FavouriteNotebookStoreImpl struct {
	db *gorm.DB
}

var _ database.FavouriteNotebookStore = &FavouriteNotebookStoreImpl{}

func NewFavouriteNotebookStoreImpl(db *gorm.DB) *FavouriteNotebookStoreImpl {
	return &FavouriteNotebookStoreImpl{db: db}
}

// InsertIgnore 首次插入；冲突时不更新 favourited_at（幂等重加保持首次时间）。
func (s *FavouriteNotebookStoreImpl) InsertIgnore(ctx context.Context, row *schema.FavouriteNotebook) error {
	cl := clause.OnConflict{
		Columns:   []clause.Column{{Name: "favourite_id"}, {Name: "notebook_id"}},
		DoNothing: true,
	}
	if err := dbFromContext(ctx, s.db).
		Clauses(cl).
		Create(row).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *FavouriteNotebookStoreImpl) Get(
	ctx context.Context,
	favouriteId, notebookId database.Id,
) (*schema.FavouriteNotebook, error) {
	var row schema.FavouriteNotebook
	err := dbFromContext(ctx, s.db).
		Where("favourite_id = ? AND notebook_id = ?", favouriteId, notebookId).
		Take(&row).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return &row, nil
}

func (s *FavouriteNotebookStoreImpl) ListByFavouriteId(
	ctx context.Context,
	favouriteId database.Id,
	limit, offset int,
) ([]*schema.FavouriteNotebook, error) {
	var rows []*schema.FavouriteNotebook
	err := dbFromContext(ctx, s.db).
		Where("favourite_id = ?", favouriteId).
		Order("favourited_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *FavouriteNotebookStoreImpl) ListByNotebookIdAndFavouriteIds(
	ctx context.Context,
	notebookId database.Id,
	favouriteIds []database.Id,
) ([]*schema.FavouriteNotebook, error) {
	if len(favouriteIds) == 0 {
		return []*schema.FavouriteNotebook{}, nil
	}
	var rows []*schema.FavouriteNotebook
	err := dbFromContext(ctx, s.db).
		Where("notebook_id = ? AND favourite_id IN ?", notebookId, favouriteIds).
		Order("favourited_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	if rows == nil {
		rows = []*schema.FavouriteNotebook{}
	}
	return rows, nil
}

func (s *FavouriteNotebookStoreImpl) CountByFavouriteId(
	ctx context.Context,
	favouriteId database.Id,
) (int64, error) {
	var count int64
	err := dbFromContext(ctx, s.db).
		Model(&schema.FavouriteNotebook{}).
		Where("favourite_id = ?", favouriteId).
		Count(&count).Error
	if err != nil {
		return 0, sql.WrapErr(err)
	}
	return count, nil
}

func (s *FavouriteNotebookStoreImpl) Delete(
	ctx context.Context,
	favouriteId database.Id,
	notebookIds []database.Id,
) error {
	if len(notebookIds) == 0 {
		return nil
	}
	if err := dbFromContext(ctx, s.db).
		Where("favourite_id = ? AND notebook_id IN ?", favouriteId, notebookIds).
		Delete(&schema.FavouriteNotebook{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *FavouriteNotebookStoreImpl) DeleteByFavouriteId(
	ctx context.Context,
	favouriteId database.Id,
) error {
	if err := dbFromContext(ctx, s.db).
		Where("favourite_id = ?", favouriteId).
		Delete(&schema.FavouriteNotebook{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *FavouriteNotebookStoreImpl) DeleteByUserId(ctx context.Context, userId valobj.Uid) error {
	// 先按用户收藏夹 id 清关联（子查询），无 FK 也能一次清干净。
	sub := dbFromContext(ctx, s.db).
		Model(&schema.Favourite{}).
		Select("id").
		Where("user_id = ?", userId)
	if err := dbFromContext(ctx, s.db).
		Where("favourite_id IN (?)", sub).
		Delete(&schema.FavouriteNotebook{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
