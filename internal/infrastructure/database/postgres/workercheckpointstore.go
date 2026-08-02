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
	updates := make(map[string]any, 9)
	if cp.Field1 != nil {
		updates["field1"] = cp.Field1
	}
	if cp.Field2 != nil {
		updates["field2"] = cp.Field2
	}
	if cp.Field3 != nil {
		updates["field3"] = cp.Field3
	}
	if cp.Field4 != nil {
		updates["field4"] = cp.Field4
	}
	if cp.Field5 != nil {
		updates["field5"] = cp.Field5
	}
	if cp.Field6 != nil {
		updates["field6"] = cp.Field6
	}
	if cp.Field7 != nil {
		updates["field7"] = cp.Field7
	}
	if cp.Field8 != nil {
		updates["field8"] = cp.Field8
	}
	if cp.UpdatedAt > 0 {
		updates["updated_at"] = cp.UpdatedAt
	}
	if len(updates) == 0 {
		return nil
	}

	err := s.db.WithContext(ctx).
		Model(&schema.WorkerCheckpoint{}).
		Where("artifact_id = ?", cp.ArtifactId).
		Updates(updates).Error
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
