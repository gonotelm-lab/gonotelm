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

type NotebookMemberStoreImpl struct {
	db *gorm.DB
}

var _ database.NotebookMemberStore = &NotebookMemberStoreImpl{}

func NewNotebookMemberStoreImpl(db *gorm.DB) *NotebookMemberStoreImpl {
	return &NotebookMemberStoreImpl{db: db}
}

func (s *NotebookMemberStoreImpl) Upsert(
	ctx context.Context,
	member *schema.NotebookMember,
) error {
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "notebook_id"}, {Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"role":       member.Role,
			"updated_at": member.UpdatedAt,
		}),
	}
	if err := s.db.WithContext(ctx).
		Clauses(cl).
		Create(member).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookMemberStoreImpl) GetByNotebookIdAndUserId(
	ctx context.Context,
	notebookId database.Id,
	userId valobj.Uid,
) (*schema.NotebookMember, error) {
	var member schema.NotebookMember
	err := s.db.WithContext(ctx).
		Where("notebook_id = ? AND user_id = ?", notebookId, userId).
		Take(&member).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &member, nil
}

func (s *NotebookMemberStoreImpl) ListByNotebookId(
	ctx context.Context,
	notebookId database.Id,
) ([]*schema.NotebookMember, error) {
	var rows []*schema.NotebookMember
	err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Order("created_at ASC").
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *NotebookMemberStoreImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
) ([]*schema.NotebookMember, error) {
	var rows []*schema.NotebookMember
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userId).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *NotebookMemberStoreImpl) Delete(
	ctx context.Context,
	notebookId database.Id,
	userId valobj.Uid,
) error {
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ? AND user_id = ?", notebookId, userId).
		Delete(&schema.NotebookMember{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookMemberStoreImpl) DeleteByNotebookId(
	ctx context.Context,
	notebookId database.Id,
) error {
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Delete(&schema.NotebookMember{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}
