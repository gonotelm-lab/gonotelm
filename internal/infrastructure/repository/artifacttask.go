package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
)

type ArtifactTaskRepository struct {
	taskStore dal.ArtifactTaskStore
}

func NewArtifactTaskRepository(taskStore dal.ArtifactTaskStore) *ArtifactTaskRepository {
	return &ArtifactTaskRepository{
		taskStore: taskStore,
	}
}

func (r *ArtifactTaskRepository) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.taskStore.DeleteByNotebookId(ctx, notebookId)
}
