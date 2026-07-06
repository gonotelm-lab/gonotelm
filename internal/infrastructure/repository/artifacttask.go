package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
)

type ArtifactTaskRepository struct {
	taskStore database.ArtifactTaskStore
}

func NewArtifactTaskRepository(taskStore database.ArtifactTaskStore) *ArtifactTaskRepository {
	return &ArtifactTaskRepository{
		taskStore: taskStore,
	}
}

func (r *ArtifactTaskRepository) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.taskStore.DeleteByNotebookId(ctx, notebookId)
}
