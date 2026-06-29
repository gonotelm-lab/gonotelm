package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
)

type Repository interface {
	Save(ctx context.Context, ns *notebook.Notebook) error
	FindById(ctx context.Context, id valobj.Id) (*notebook.Notebook, error)
	ListByOwner(ctx context.Context, ownerId string, spec *ListSpec) ([]*notebook.Notebook, error)
}
