package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	sourceservice "github.com/gonotelm-lab/gonotelm/internal/domain/source/service/source"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListSourcesHandler struct {
	notebookRepo  notebookrepo.Repository
	sourceRepo    sourcerepo.Repository
	sourceService sourceservice.Service
	storageRepo   sourcerepo.StorageRepository
}

func NewListSourcesHandler(
	notebookRepo notebookrepo.Repository,
	sourceRepo sourcerepo.Repository,
	storageRepo sourcerepo.StorageRepository,
) *ListSourcesHandler {
	return &ListSourcesHandler{
		notebookRepo:  notebookRepo,
		sourceRepo:    sourceRepo,
		sourceService: sourceservice.New(storageRepo),
	}
}

type ListSourcesQuery struct {
	NotebookId valobj.Id
	Limit      int
	Offset     int
}

type ListSourcesResult struct {
	Sources []*sourceentity.SourceDetail
	Limit   int
	Offset  int
	HasMore bool
}

func (h *ListSourcesHandler) Handle(ctx context.Context, query *ListSourcesQuery) (*ListSourcesResult, error) {
	_, err := h.notebookRepo.FindById(ctx, query.NotebookId)
	if err != nil {
		return nil, errors.WithMessage(err, "find notebook by id failed")
	}

	sources, err := h.sourceRepo.ListByNotebookId(ctx, query.NotebookId,
		&sourcerepo.ListSpec{
			Offset: query.Offset,
			Limit:  query.Limit + 1,
		})
	if err != nil {
		return nil, errors.WithMessage(err, "list sources failed")
	}

	hasMore := len(sources) > query.Limit
	if hasMore {
		sources = sources[:query.Limit]
	}

	result, err := h.sourceService.ListSourcesDetail(ctx, sources)
	if err != nil {
		return nil, errors.WithMessage(err, "list sources detail failed")
	}

	return &ListSourcesResult{
		Sources: result,
		Limit:   query.Limit,
		Offset:  query.Offset,
		HasMore: len(sources) > query.Limit,
	}, nil
}
