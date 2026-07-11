package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Repository interface {
	Save(ctx context.Context, artifact *entity.Artifact) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Artifact, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id, limit, offset int) ([]*entity.Artifact, error)
	ListByStatus(ctx context.Context, statuses []entity.Status, limit int) ([]*entity.Artifact, error)
	UpdateStatus(ctx context.Context, id valobj.Id, status entity.Status, result []byte, resultKind entity.ResultKind, title string) error
	UpdateFlowTaskId(ctx context.Context, id valobj.Id, flowTaskId string, oldStatuses []entity.Status) error
	DeleteById(ctx context.Context, id valobj.Id) error
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
