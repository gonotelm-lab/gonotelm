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

func (s *ArtifactStoreImpl) GetById(ctx context.Context, id database.Id) (*schema.Artifact, error) {
	var a schema.Artifact
	if err := s.db.WithContext(ctx).Where("id = ?", id).Take(&a).Error; err != nil {
		return nil, sql.WrapErr(err)
	}
	return &a, nil
}

func (s *ArtifactStoreImpl) GetStatusById(ctx context.Context, id database.Id) (string, error) {
	var a schema.Artifact
	if err := s.db.WithContext(ctx).Model(&schema.Artifact{}).Where("id = ?", id).Select("status").Take(&a).Error; err != nil {
		return "", sql.WrapErr(err)
	}
	return a.Status, nil
}

func (s *ArtifactStoreImpl) ListByNotebookId(ctx context.Context, notebookId database.Id, limit, offset int) ([]*schema.Artifact, error) {
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

func (s *ArtifactStoreImpl) ListByStatus(ctx context.Context, statuses []string, limit int) ([]*schema.Artifact, error) {
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

func (s *ArtifactStoreImpl) UpdateStatus(
	ctx context.Context, id database.Id, newStatus string, oldStatus string, params *schema.ArtifactUpdateStatusParams,
) (bool, error) {
	updates := map[string]any{"status": newStatus, "updated_at": params.UpdatedAt}
	if params.Title != "" {
		updates["title"] = params.Title
	}
	if params.Result != nil {
		updates["result"] = params.Result
	}
	if params.ResultKind != "" {
		updates["result_kind"] = params.ResultKind
	}
	q := s.db.WithContext(ctx).
		Model(&schema.Artifact{}).
		Where("id = ?", id)
	if oldStatus != "" {
		q = q.Where("status = ?", oldStatus)
	}
	res := q.Updates(updates)
	if res.Error != nil {
		return false, sql.WrapErr(res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (s *ArtifactStoreImpl) UpdateFlowTaskId(ctx context.Context, id database.Id, flowTaskId string, oldStatuses []string) error {
	q := s.db.WithContext(ctx).Model(&schema.Artifact{}).Where("id = ?", id)
	if len(oldStatuses) > 0 {
		q = q.Where("status IN ?", oldStatuses)
	}
	if err := q.Updates(map[string]any{"flow_task_id": flowTaskId, "status": "pending", "updated_at": gorm.Expr("now()")}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
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