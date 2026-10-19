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

type NotebookStoreImpl struct {
	db *gorm.DB
}

var _ database.NotebookStore = &NotebookStoreImpl{}

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

func (s *NotebookStoreImpl) Upsert(
	ctx context.Context,
	notebook *schema.Notebook,
) error {
	// insert into xxx on conflict do update set ...
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"name":        notebook.Name,
			"description": notebook.Description,
			"visibility":  notebook.Visibility,
			"updated_at":  notebook.UpdatedAt,
		}),
	}
	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Clauses(cl).
		Create(notebook).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *NotebookStoreImpl) Exists(ctx context.Context, id database.Id) (bool, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("id = ?", id).
		Count(&count).Error; err != nil {
		return false, sql.WrapErr(err)
	}

	return count > 0, nil
}

func (s *NotebookStoreImpl) GetById(
	ctx context.Context, id database.Id,
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
	name string,
	ownerId valobj.Uid,
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
	ownerId valobj.Uid,
) ([]*schema.Notebook, error) {
	var rows []*schema.Notebook
	err := s.db.WithContext(ctx).
		Where("owner_id = ?", ownerId).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *NotebookStoreImpl) ListByIds(
	ctx context.Context,
	ids []database.Id,
) ([]*schema.Notebook, error) {
	if len(ids) == 0 {
		return []*schema.Notebook{}, nil
	}

	var rows []*schema.Notebook
	err := s.db.WithContext(ctx).
		Where("id IN ?", ids).
		Find(&rows).Error
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
			"visibility":  notebook.Visibility,
			"updated_at":  notebook.UpdatedAt,
		}).Error
	if err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *NotebookStoreImpl) DeleteById(ctx context.Context, id valobj.Id) error {
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
