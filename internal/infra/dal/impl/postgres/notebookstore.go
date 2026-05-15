package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type NotebookStoreImpl struct {
	db *gorm.DB
}

var _ dal.NotebookStore = &NotebookStoreImpl{}

func NewNotebookStoreImpl(db *gorm.DB) *NotebookStoreImpl {
	return &NotebookStoreImpl{db: db}
}

func (s *NotebookStoreImpl) Create(ctx context.Context, notebook *schema.Notebook) error {
	if err := s.db.WithContext(ctx).Create(notebook).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.Notebook, error) {
	var notebook schema.Notebook
	err := s.db.WithContext(ctx).
		Where("id = ?", id).
		Take(&notebook).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &notebook, nil
}

func (s *NotebookStoreImpl) GetByNameAndOwnerId(
	ctx context.Context,
	name, ownerId string,
) (*schema.Notebook, error) {
	var notebook schema.Notebook
	err := s.db.WithContext(ctx).
		Where("name = ? AND owner_id = ?", name, ownerId).
		Take(&notebook).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &notebook, nil
}

func (s *NotebookStoreImpl) ListByOwnerId(
	ctx context.Context,
	ownerId string,
	limit, offset, orderBy int,
) ([]*schema.Notebook, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	query := s.db.WithContext(ctx).
		Where("owner_id = ?", ownerId)
	if orderBy == 0 {
		query = query.Order("id ASC")
	} else {
		query = query.Order("updated_at DESC")
	}

	var rows []*schema.Notebook
	err := query.Limit(limit).Offset(offset).Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *NotebookStoreImpl) Update(ctx context.Context, notebook *schema.Notebook) error {
	err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("id = ?", notebook.Id).
		Updates(map[string]any{
			"name":        notebook.Name,
			"description": notebook.Description,
			"owner_id":    notebook.OwnerId,
			"updated_at":  notebook.UpdatedAt,
		}).Error
	if err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *NotebookStoreImpl) DeleteById(ctx context.Context, id dal.Id) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&schema.Notebook{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookStoreImpl) UpdateName(ctx context.Context, id dal.Id, name string) error {
	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("id = ?", id).
		Update("name", name).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *NotebookStoreImpl) UpdateDesc(ctx context.Context, id dal.Id, desc string) error {
	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("id = ?", id).
		Update("description", desc).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
