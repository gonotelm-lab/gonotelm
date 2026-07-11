package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Repository interface {
	Save(ctx context.Context, artifact *entity.Artifact) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Artifact, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id, spec *ListSpec) ([]*entity.Artifact, error)
	ListByStatus(ctx context.Context, spec *ListByStatusSpec) ([]*entity.Artifact, error)
	DeleteById(ctx context.Context, id valobj.Id) error
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
