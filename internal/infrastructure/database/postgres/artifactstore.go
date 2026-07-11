package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

func (s *ArtifactStoreImpl) Upsert(ctx context.Context, a *schema.Artifact) error {
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status":       a.Status,
			"flow_task_id":  a.FlowTaskId,
			"title":         a.Title,
			"result":        a.Result,
			"result_kind":   a.ResultKind,
			"payload":       a.Payload,
			"updated_at":     a.UpdatedAt,
		}),
	}
	if err := s.db.WithContext(ctx).
		Model(&schema.Artifact{}).
		Clauses(cl).
		Create(a).Error; err != nil {
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
