package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type ArtifactStoreImpl struct{ db *gorm.DB }

var _ database.ArtifactStore = &ArtifactStoreImpl{}

func NewArtifactStoreImpl(db *gorm.DB) *ArtifactStoreImpl {
	return &ArtifactStoreImpl{db: db}
}

func (s *ArtifactStoreImpl) Create(ctx context.Context, a *schema.Artifact) error {
	if err := s.db.WithContext(ctx).Create(a).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

// UpdateExisting 只更新已存在的行，返回受影响行数
func (s *ArtifactStoreImpl) UpdateExisting(ctx context.Context, a *schema.Artifact) (int64, error) {
	result := s.db.WithContext(ctx).
		Model(&schema.Artifact{}).
		Where("id = ?", a.Id).
		Updates(map[string]any{
			"status":       a.Status,
			"flow_task_id": a.FlowTaskId,
			"title":        a.Title,
			"result":       a.Result,
			"result_kind":  a.ResultKind,
			"payload":      a.Payload,
			"updated_at":   a.UpdatedAt,
		})
	if result.Error != nil {
		return 0, sql.WrapErr(result.Error)
	}

	return result.RowsAffected, nil
}

func (s *ArtifactStoreImpl) GetById(ctx context.Context, id database.Id) (*schema.Artifact, error) {
	var a schema.Artifact
	if err := s.db.WithContext(ctx).Where("id = ?", id).Take(&a).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return &a, nil
}

func (s *ArtifactStoreImpl) PageListByNotebookId(ctx context.Context, notebookId database.Id, limit, offset int) ([]*schema.Artifact, error) {
	var rows []*schema.Artifact
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Order("created_at DESC, id DESC").
		Limit(limit).Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ArtifactStoreImpl) PageListByStatus(ctx context.Context, statuses []string, limit int) ([]*schema.Artifact, error) {
	var rows []*schema.Artifact
	if err := s.db.WithContext(ctx).
		Where("status IN ?", statuses).
		Order("updated_at ASC, id ASC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ArtifactStoreImpl) DeleteById(ctx context.Context, id database.Id) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&schema.Artifact{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ArtifactStoreImpl) DeleteByNotebookId(ctx context.Context, notebookId database.Id) error {
	if err := s.db.WithContext(ctx).Where("notebook_id = ?", notebookId).Delete(&schema.Artifact{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
