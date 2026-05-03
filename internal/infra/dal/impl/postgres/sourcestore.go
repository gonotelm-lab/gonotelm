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
	tx := s.db.WithContext(ctx).Exec(
		"INSERT INTO sources (id, notebook_id, kind, status, display_name, content, owner_id, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		source.Id, source.NotebookId, source.Kind, source.Status, source.DisplayName, source.Content, source.OwnerId, source.UpdatedAt,
	)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *SourceStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.Source, error) {
	source, err := gorm.G[*schema.Source](s.db).
		Raw(
			"SELECT id, notebook_id, kind, status, display_name, content, owner_id, updated_at FROM sources WHERE id = ? LIMIT 1",
			id,
		).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return source, nil
}

func (s *SourceStoreImpl) CountByNotebookId(
	ctx context.Context,
	notebookId dal.Id,
) (int64, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Raw(
			"SELECT COUNT(1) FROM sources WHERE notebook_id = ? AND status <> ?",
			notebookId,
			schema.SourceStatusInited,
		).
		Scan(&count).Error
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

	rows, err := gorm.G[*schema.Source](s.db).
		Raw(
			"SELECT id, notebook_id, kind, status, display_name, content, owner_id, updated_at FROM sources WHERE notebook_id = ? AND status <> ? ORDER BY updated_at DESC LIMIT ? OFFSET ?",
			notebookId,
			schema.SourceStatusInited,
			limit,
			offset,
		).
		Find(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *SourceStoreImpl) DeleteById(ctx context.Context, id dal.Id) error {
	tx := s.db.WithContext(ctx).Exec("DELETE FROM sources WHERE id = ?", id)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *SourceStoreImpl) DeleteByNotebookId(ctx context.Context, notebookId dal.Id) error {
	tx := s.db.WithContext(ctx).Exec(
		"DELETE FROM sources WHERE notebook_id = ?",
		notebookId,
	)

	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *SourceStoreImpl) UpdateStatus(ctx context.Context, id dal.Id, status string) error {
	tx := s.db.WithContext(ctx).Exec(
		"UPDATE sources SET status = ? WHERE id = ?",
		status, id)

	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *SourceStoreImpl) Update(ctx context.Context, params *schema.SourceUpdateParams) error {
	tx := s.db.WithContext(ctx).Exec(
		"UPDATE sources SET status = ?, display_name = ?, content = ?, updated_at = ? WHERE id = ?",
		params.Status, params.DisplayName, params.Content, params.UpdatedAt, params.Id)

	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *SourceStoreImpl) ListByIds(ctx context.Context, ids []dal.Id) ([]*schema.Source, error) {
	return batch.BatchMap(
		ctx,
		ids,
		sourceIDsQueryBatchSize,
		func(ctx context.Context, batch []dal.Id) ([]*schema.Source, error) {
			rows, err := gorm.G[*schema.Source](s.db).
				Raw(
					"SELECT id, notebook_id, kind, status, display_name, content, owner_id, updated_at FROM sources WHERE id IN ?",
					batch,
				).
				Find(ctx)
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
			rows, err := gorm.G[*schema.Source](s.db).
				Raw(
					"SELECT id, notebook_id, kind, status, display_name, content, owner_id, updated_at FROM sources WHERE notebook_id = ? AND id IN ?",
					notebookId,
					batch,
				).
				Find(ctx)
			if err != nil {
				return nil, sql.WrapErr(err)
			}

			return rows, nil
		},
	)
}
