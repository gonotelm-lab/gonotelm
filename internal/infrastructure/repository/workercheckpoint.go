package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	workerrepo "github.com/gonotelm-lab/gonotelm/internal/domain/worker/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
)

type CheckpointRepositoryImpl struct {
	store database.WorkerCheckpointStore
}

func NewCheckpointRepository(store database.WorkerCheckpointStore) workerrepo.CheckpointRepository {
	return &CheckpointRepositoryImpl{store: store}
}

var _ workerrepo.CheckpointRepository = &CheckpointRepositoryImpl{}

func (r *CheckpointRepositoryImpl) FindByArtifactId(ctx context.Context, artifactId valobj.Id) (*entity.Checkpoint, error) {
	sch, err := r.store.GetByArtifactId(ctx, artifactId)
	if err != nil {
		return nil, err
	}

	return mapper.CheckpointFromSchema(sch), nil
}

func (r *CheckpointRepositoryImpl) DeleteByArtifactId(ctx context.Context, artifactId valobj.Id) error {
	return r.store.DeleteByArtifactId(ctx, artifactId)
}

func (r *CheckpointRepositoryImpl) Save(ctx context.Context, cp *entity.Checkpoint) error {
	_, err := r.store.GetByArtifactId(ctx, cp.ArtifactId)
	if err != nil {
		return r.store.Create(ctx, mapper.CheckpointToSchema(cp))
	}

	return r.store.Update(ctx, mapper.CheckpointToSchema(cp))
}
