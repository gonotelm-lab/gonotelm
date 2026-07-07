package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookentity "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/entity"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type NotebookRepositoryImpl struct {
	notebookStore database.NotebookStore
	sourceStore   database.SourceStore
}

func NewNotebookRepository(
	notebookStore database.NotebookStore,
	sourceStore database.SourceStore,
) notebookrepo.Repository {
	return &NotebookRepositoryImpl{
		notebookStore: notebookStore,
		sourceStore:   sourceStore,
	}
}

var _ notebookrepo.Repository = &NotebookRepositoryImpl{}

func (s *NotebookRepositoryImpl) Save(ctx context.Context, notebook *notebookentity.Notebook) error {
	if notebook.IsDeleted() {
		return s.notebookStore.DeleteById(ctx, notebook.Id)
	}

	sch := mapper.NotebookToSchema(notebook)
	return s.notebookStore.Upsert(ctx, sch)
}

func (s *NotebookRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*notebookentity.Notebook, error) {
	notebook, err := s.notebookStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, notebookerrors.ErrNotebookNotFound
		}

		return nil, err
	}

	sourceCount, err := s.sourceStore.CountByNotebookId(ctx, id)
	if err != nil {
		return nil, err
	}

	result := mapper.NotebookFromSchema(notebook)
	result.SourceCount = sourceCount

	return result, nil
}

func (s *NotebookRepositoryImpl) ListByOwner(
	ctx context.Context,
	ownerId string,
	spec *notebookrepo.ListSpec,
) ([]*notebookentity.Notebook, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	orderBy := 0
	if spec.Order == notebookrepo.ListSpecOrderUpdateTime {
		orderBy = 1
	}

	rows, err := s.notebookStore.ListByOwnerId(
		ctx, ownerId, spec.Limit, spec.Offset, orderBy,
	)
	if err != nil {
		return nil, err
	}

	notebookIds := make([]valobj.Id, 0, len(rows))
	for _, row := range rows {
		notebookIds = append(notebookIds, row.Id)
	}

	counts, err := s.sourceStore.BatchCountByNotebookIds(ctx, notebookIds)
	if err != nil {
		return nil, err
	}

	notebooks := make([]*notebookentity.Notebook, 0, len(rows))
	for _, row := range rows {
		notebook := mapper.NotebookFromSchema(row)
		notebook.SourceCount = counts[row.Id]
		notebooks = append(notebooks, notebook)
	}

	return notebooks, nil
}
