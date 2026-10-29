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

type UserNotebookPinStoreImpl struct {
	db *gorm.DB
}

var _ database.UserNotebookPinStore = &UserNotebookPinStoreImpl{}

func NewUserNotebookPinStoreImpl(db *gorm.DB) *UserNotebookPinStoreImpl {
	return &UserNotebookPinStoreImpl{db: db}
}

func (s *UserNotebookPinStoreImpl) Upsert(
	ctx context.Context,
	pin *schema.UserNotebookPin,
) error {
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "notebook_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"pinned_at": pin.PinnedAt,
		}),
	}
	if err := s.db.WithContext(ctx).
		Clauses(cl).
		Create(pin).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserNotebookPinStoreImpl) Get(
	ctx context.Context,
	userId valobj.Uid,
	notebookId database.Id,
) (*schema.UserNotebookPin, error) {
	var pin schema.UserNotebookPin
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND notebook_id = ?", userId, notebookId).
		Take(&pin).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &pin, nil
}

func (s *UserNotebookPinStoreImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
) ([]*schema.UserNotebookPin, error) {
	var rows []*schema.UserNotebookPin
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Order("pinned_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *UserNotebookPinStoreImpl) CountByUserId(
	ctx context.Context,
	userId valobj.Uid,
) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&schema.UserNotebookPin{}).
		Where("user_id = ?", userId).
		Count(&count).Error
	if err != nil {
		return 0, sql.WrapErr(err)
	}

	return count, nil
}

func (s *UserNotebookPinStoreImpl) Delete(
	ctx context.Context,
	userId valobj.Uid,
	notebookId database.Id,
) error {
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND notebook_id = ?", userId, notebookId).
		Delete(&schema.UserNotebookPin{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserNotebookPinStoreImpl) DeleteByNotebookId(
	ctx context.Context,
	notebookId database.Id,
) error {
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Delete(&schema.UserNotebookPin{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}
