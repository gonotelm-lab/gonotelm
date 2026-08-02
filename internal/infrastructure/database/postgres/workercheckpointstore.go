package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type WorkerCheckpointStoreImpl struct{ db *gorm.DB }

var _ database.WorkerCheckpointStore = &WorkerCheckpointStoreImpl{}

func NewWorkerCheckpointStoreImpl(db *gorm.DB) *WorkerCheckpointStoreImpl {
	return &WorkerCheckpointStoreImpl{db: db}
}

func (s *WorkerCheckpointStoreImpl) Create(ctx context.Context, cp *schema.WorkerCheckpoint) error {
	err := s.db.WithContext(ctx).Create(cp).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *WorkerCheckpointStoreImpl) GetByArtifactId(
	ctx context.Context, artifactId database.Id,
) (*schema.WorkerCheckpoint, error) {
	var cp schema.WorkerCheckpoint
	err := s.db.WithContext(ctx).
		Where("artifact_id = ?", artifactId).
		Take(&cp).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &cp, nil
}

func (s *WorkerCheckpointStoreImpl) Update(ctx context.Context, cp *schema.WorkerCheckpoint) error {
	err := s.db.WithContext(ctx).
		Model(&schema.WorkerCheckpoint{}).
		Where("artifact_id = ?", cp.ArtifactId).
		Updates(map[string]any{
			"field1":     cp.Field1,
			"field2":     cp.Field2,
			"field3":     cp.Field3,
			"field4":     cp.Field4,
			"field5":     cp.Field5,
			"field6":     cp.Field6,
			"field7":     cp.Field7,
			"field8":     cp.Field8,
			"updated_at": cp.UpdatedAt,
		}).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *WorkerCheckpointStoreImpl) DeleteByArtifactId(
	ctx context.Context, artifactId database.Id,
) error {
	err := s.db.WithContext(ctx).
		Where("artifact_id = ?", artifactId).
		Delete(&schema.WorkerCheckpoint{}).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}
