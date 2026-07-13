package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
)

type CheckpointRepository interface {
	FindByArtifactId(ctx context.Context, artifactId valobj.Id) (*entity.Checkpoint, error)
	DeleteByArtifactId(ctx context.Context, artifactId valobj.Id) error
	Save(ctx context.Context, checkpoint *entity.Checkpoint) error
}
