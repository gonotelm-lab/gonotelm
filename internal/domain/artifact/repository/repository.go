package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Repository interface {
	Create(ctx context.Context, artifact *entity.Artifact) error
	// UpdateExisting 只更新已存在的记录
	UpdateExisting(ctx context.Context, artifact *entity.Artifact) (int64, error)
	FindById(ctx context.Context, id valobj.Id) (*entity.Artifact, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id, spec *ListSpec) ([]*entity.Artifact, error)
	ListByStatus(ctx context.Context, spec *ListByStatusSpec) ([]*entity.Artifact, error)
	DeleteById(ctx context.Context, id valobj.Id) error
	DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error
}
