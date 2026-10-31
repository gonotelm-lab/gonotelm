package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type SourceRepositoryImpl struct {
	sourceStore database.SourceStore
}

func NewSourceRepository(sourceStore database.SourceStore) sourcerepo.Repository {
	return &SourceRepositoryImpl{
		sourceStore: sourceStore,
	}
}

var _ sourcerepo.Repository = &SourceRepositoryImpl{}

func (s *SourceRepositoryImpl) Create(ctx context.Context, source *sourceentity.Source) error {
	return s.sourceStore.Create(ctx, mapper.SourceToSchema(source))
}

func (s *SourceRepositoryImpl) UpdateExisting(
	ctx context.Context,
	source *sourceentity.Source,
) (int64, error) {
	return s.sourceStore.UpdateExisting(ctx, mapper.SourceToSchema(source))
}

func (s *SourceRepositoryImpl) DeleteById(ctx context.Context, id valobj.Id) error {
	return s.sourceStore.DeleteById(ctx, id)
}

func (s *SourceRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*sourceentity.Source, error) {
	source, err := s.sourceStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, domainerr.ErrSourceNotFound
		}

		return nil, err
	}

	return mapper.SourceFromSchema(source)
}

func (s *SourceRepositoryImpl) PageListByNotebookId(
	ctx context.Context,
	notebookId valobj.Id,
	spec *sourcerepo.PageListSpec,
) ([]*sourceentity.Source, error) {
	if err := spec.Validate(); err != nil {
		return nil, err
	}

	sources, err := s.sourceStore.PageListByNotebookId(ctx, notebookId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}

	return mapper.SourcesFromSchemas(sources)
}

func (s *SourceRepositoryImpl) GetByNotebookIdAndIds(
	ctx context.Context,
	notebookId valobj.Id,
	ids []valobj.Id,
) ([]*sourceentity.Source, error) {
	sources, err := s.sourceStore.ListByNotebookIdAndIds(ctx, notebookId, ids)
	if err != nil {
		return nil, err
	}

	return mapper.SourcesFromSchemas(sources)
}

func (s *SourceRepositoryImpl) BatchDeleteByIds(ctx context.Context, ids []valobj.Id) error {
	if len(ids) == 0 {
		return nil
	}
	return s.sourceStore.BatchDelete(ctx, ids)
}

func (s *SourceRepositoryImpl) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return s.sourceStore.DeleteByNotebookId(ctx, notebookId)
}
