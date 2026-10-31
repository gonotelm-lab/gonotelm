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

type FavouriteStoreImpl struct {
	db *gorm.DB
}

var _ database.FavouriteStore = &FavouriteStoreImpl{}

func NewFavouriteStoreImpl(db *gorm.DB) *FavouriteStoreImpl {
	return &FavouriteStoreImpl{db: db}
}

func (s *FavouriteStoreImpl) Upsert(ctx context.Context, favourite *schema.Favourite) error {
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name":       favourite.Name,
			"updated_at": favourite.UpdatedAt,
		}),
	}
	if err := dbFromContext(ctx, s.db).
		Clauses(cl).
		Create(favourite).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *FavouriteStoreImpl) GetById(ctx context.Context, id database.Id) (*schema.Favourite, error) {
	var row schema.Favourite
	err := dbFromContext(ctx, s.db).
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return &row, nil
}

func (s *FavouriteStoreImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
	limit, offset int,
) ([]*schema.Favourite, error) {
	var rows []*schema.Favourite
	err := dbFromContext(ctx, s.db).
		Where("user_id = ?", userId).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *FavouriteStoreImpl) CountByUserId(ctx context.Context, userId valobj.Uid) (int64, error) {
	var count int64
	err := dbFromContext(ctx, s.db).
		Model(&schema.Favourite{}).
		Where("user_id = ?", userId).
		Count(&count).Error
	if err != nil {
		return 0, sql.WrapErr(err)
	}
	return count, nil
}

func (s *FavouriteStoreImpl) DeleteById(ctx context.Context, id database.Id) error {
	if err := dbFromContext(ctx, s.db).
		Where("id = ?", id).
		Delete(&schema.Favourite{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *FavouriteStoreImpl) DeleteByUserId(ctx context.Context, userId valobj.Uid) error {
	if err := dbFromContext(ctx, s.db).
		Where("user_id = ?", userId).
		Delete(&schema.Favourite{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
