package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type SourceStoreImpl struct {
	db *gorm.DB
}

var sourceIDsQueryBatchSize = 1000

var _ dal.SourceStore = &SourceStoreImpl{}

func NewSourceStoreImpl(db *gorm.DB) *SourceStoreImpl {
	return &SourceStoreImpl{db: db}
}

func (s *SourceStoreImpl) Create(ctx context.Context, source *schema.Source) error {
	if err := s.db.WithContext(ctx).Create(source).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *SourceStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.Source, error) {
	var source schema.Source
	err := s.db.WithContext(ctx).
		Where("id = ?", id).
		Take(&source).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &source, nil
}

func (s *SourceStoreImpl) CountByNotebookId(
	ctx context.Context,
	notebookId dal.Id,
) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&schema.Source{}).
		Where("notebook_id = ? AND status <> ?", notebookId, schema.SourceStatusInited).
		Count(&count).Error
	if err != nil {
		return 0, sql.WrapErr(err)
	}

	return count, nil
}

func (s *SourceStoreImpl) ListByNotebookId(
	ctx context.Context,
	notebookId dal.Id,
	limit, offset int,
) ([]*schema.Source, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	var rows []*schema.Source
	err := s.db.WithContext(ctx).
		Where("notebook_id = ? AND status <> ?", notebookId, schema.SourceStatusInited).
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *SourceStoreImpl) DeleteById(ctx context.Context, id dal.Id) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&schema.Source{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *SourceStoreImpl) DeleteByNotebookId(ctx context.Context, notebookId dal.Id) error {
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Delete(&schema.Source{}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *SourceStoreImpl) UpdateStatus(
	ctx context.Context,
	params *schema.SourceUpdateStatusParams,
) error {
	if err := s.db.WithContext(ctx).
		Model(&schema.Source{}).
		Where("id = ?", params.Id).
		Updates(map[string]any{
			"status":     params.Status,
			"updated_at": params.UpdatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *SourceStoreImpl) Update(ctx context.Context, params *schema.SourceUpdateParams) error {
	err := s.db.WithContext(ctx).
		Model(&schema.Source{}).
		Where("id = ?", params.Id).
		Updates(map[string]any{
			"status":       params.Status,
			"display_name": params.DisplayName,
			"content":      params.Content,
			"updated_at":   params.UpdatedAt,
		}).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *SourceStoreImpl) UpdateParsedContent(
	ctx context.Context,
	params *schema.SourceUpdateParsedContentParams,
) error {
	if err := s.db.WithContext(ctx).
		Model(&schema.Source{}).
		Where("id = ?", params.Id).
		Updates(map[string]any{
			"parsed_content": params.ParsedContent,
			"updated_at":     params.UpdatedAt,
		}).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *SourceStoreImpl) ListByIds(ctx context.Context, ids []dal.Id) ([]*schema.Source, error) {
	return batch.BatchMap(
		ctx,
		ids,
		sourceIDsQueryBatchSize,
		func(ctx context.Context, batch []dal.Id) ([]*schema.Source, error) {
			var rows []*schema.Source
			err := s.db.WithContext(ctx).
				Where("id IN ?", batch).
				Find(&rows).Error
			if err != nil {
				return nil, sql.WrapErr(err)
			}

			return rows, nil
		})
}

func (s *SourceStoreImpl) ListByNotebookIdAndIds(
	ctx context.Context,
	notebookId dal.Id,
	ids []dal.Id,
) ([]*schema.Source, error) {
	return batch.BatchMap(
		ctx,
		ids,
		sourceIDsQueryBatchSize,
		func(ctx context.Context, batch []dal.Id) ([]*schema.Source, error) {
			var rows []*schema.Source
			err := s.db.WithContext(ctx).
				Where("notebook_id = ? AND id IN ?", notebookId, batch).
				Find(&rows).Error
			if err != nil {
				return nil, sql.WrapErr(err)
			}

			return rows, nil
		},
	)
}
