package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
)

type Repository interface {
	Save(ctx context.Context, ns *notebookentity.Notebook) error
	FindById(ctx context.Context, id valobj.Id) (*notebookentity.Notebook, error)
	ListByOwner(ctx context.Context, ownerId string, spec *ListSpec) ([]*notebookentity.Notebook, error)
}
