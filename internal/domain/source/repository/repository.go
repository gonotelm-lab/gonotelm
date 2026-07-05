package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
)

type Repository interface {
	Save(ctx context.Context, s *entity.Source) error
	FindById(ctx context.Context, id valobj.Id) (*entity.Source, error)
	ListByNotebookId(ctx context.Context, notebookId valobj.Id, spec *ListSpec) ([]*entity.Source, error)
	GetByNotebookIdAndIds(ctx context.Context, notebookId valobj.Id, ids []valobj.Id) ([]*entity.Source, error)
}
