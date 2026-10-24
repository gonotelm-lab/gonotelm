package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type NotebookStoreImpl struct {
	db *gorm.DB
}

var (
	_ database.NotebookStore            = &NotebookStoreImpl{}
	_ database.NotebookMaintenanceStore = &NotebookStoreImpl{}
)

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
			"flags":       notebook.Flags,
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
	limit, offset int,
	order database.NotebookListOrder,
) ([]*schema.Notebook, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	query, err := applyNotebookListOrder(
		s.db.WithContext(ctx).Where("owner_id = ?", ownerId),
		order,
	)
	if err != nil {
		return nil, err
	}

	var rows []*schema.Notebook
	if err := query.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

// ListByIds 按 id 批量查询；ids 为空时返回空切片。
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

// ListByIdsExcludingOwner 返回 ids 中 owner 不是 excludeOwnerId 的 notebook，用于 shared 列表。
func (s *NotebookStoreImpl) ListByIdsExcludingOwner(
	ctx context.Context,
	ids []database.Id,
	excludeOwnerId valobj.Uid,
	limit, offset int,
	order database.NotebookListOrder,
) ([]*schema.Notebook, error) {
	if len(ids) == 0 {
		return []*schema.Notebook{}, nil
	}
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	query, err := applyNotebookListOrder(
		s.db.WithContext(ctx).
			Where("id IN ?", ids).
			Where("owner_id <> ?", excludeOwnerId),
		order,
	)
	if err != nil {
		return nil, err
	}

	var rows []*schema.Notebook
	if err := query.Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

// applyNotebookListOrder 只接受 database 层定义的两档排序，避免把参数拼进 SQL。
func applyNotebookListOrder(
	query *gorm.DB,
	order database.NotebookListOrder,
) (*gorm.DB, error) {
	switch order {
	case database.NotebookListOrderLastActive:
		// updated_at 同毫秒时用 id 兜底，保证翻页顺序稳定。
		return query.Order("notebooks.updated_at DESC").Order("notebooks.id DESC"), nil
	case database.NotebookListOrderCreateTime:
		return query.Order("notebooks.id ASC"), nil
	default:
		return nil, xerror.ErrParams.Msgf("invalid notebook list order: %d", order)
	}
}

func (s *NotebookStoreImpl) Update(ctx context.Context, notebook *schema.Notebook) error {
	err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("id = ?", notebook.Id).
		Updates(map[string]any{
			"name":        notebook.Name,
			"description": notebook.Description,
			"flags":       notebook.Flags,
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

func (s *NotebookStoreImpl) CountNonEmptyDescriptions(ctx context.Context) (int64, error) {
	var count int64
	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("description <> ''").
		Count(&count).Error; err != nil {
		return 0, sql.WrapErr(err)
	}
	return count, nil
}

func (s *NotebookStoreImpl) ClearNonEmptyDescriptions(
	ctx context.Context,
	params *schema.NotebookClearDescriptionsParams,
) error {
	if err := s.db.WithContext(ctx).
		Model(&schema.Notebook{}).
		Where("description <> ''").
		Updates(map[string]any{
			"description": "",
			"updated_at":  params.UpdatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
