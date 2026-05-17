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

func (s *NotebookStoreImpl) Create(
	ctx context.Context,
	notebook *schema.Notebook,
) error {
	if err := s.db.WithContext(ctx).Create(notebook).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookStoreImpl) GetById(
	ctx context.Context, id dal.Id,
) (*schema.Notebook, error) {
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
	if err := s.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&schema.Notebook{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookStoreImpl) UpdateName(
	ctx context.Context,
	params *schema.NotebookUpdateNameParams,
) error {
	where := "id = ?"
	whereParams := []any{params.Id}
	if params.SkipIfNonEmpty {
		where += " AND name = ''"
	}

	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where(where, whereParams...).
		Updates(map[string]any{
			"name":       params.Name,
			"updated_at": params.UpdatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *NotebookStoreImpl) UpdateDescription(
	ctx context.Context,
	params *schema.NotebookUpdateDescriptionParams,
) error {
	where := "id = ?"
	whereParams := []any{params.Id}
	if params.SkipIfNonEmpty {
		where += " AND description = ''"
	}

	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where(where, whereParams...).
		Updates(map[string]any{
			"description": params.Description,
			"updated_at":  params.UpdatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookStoreImpl) FillNameAndDescriptionIfEmpty(
	ctx context.Context,
	params *schema.NotebookFillNameAndDescriptionParams,
) error {
	// UPDATE "notebooks"
	// SET
	// 	"name" = CASE
	// 		WHEN "name" <> '' THEN "name"
	// 		ELSE $1
	// 	END,
	// 	"description" = CASE
	// 		WHEN "description" <> '' THEN "description"
	// 		ELSE $2
	// 	END,
	// 	"updated_at" = CASE
	// 		WHEN "name" <> '' AND "description" <> '' THEN "updated_at"
	// 		ELSE $3
	// 	END
	// WHERE "id" = $4;

	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("id = ?", params.Id).
		Updates(map[string]any{
			"name": gorm.Expr(
				"CASE WHEN name <> '' THEN name ELSE ? END",
				params.Name,
			),
			"description": gorm.Expr(
				"CASE WHEN description <> '' THEN description ELSE ? END",
				params.Description,
			),
			"updated_at": gorm.Expr(
				`CASE
					WHEN name <> '' AND description <> ''
					THEN updated_at
					ELSE ?
				END`,
				params.UpdatedAt,
			),
		}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}
