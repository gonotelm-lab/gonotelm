package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	sourceservice "github.com/gonotelm-lab/gonotelm/internal/domain/source/service/source"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ListSourcesHandler struct {
	sourceRepo            sourcerepo.Repository
	notebookAccessService *notebookservice.AccessService
	sourceService         sourceservice.Service
}

func NewListSourcesHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	objectStore adapter.ObjectStore,
) *ListSourcesHandler {
	return &ListSourcesHandler{
		sourceRepo:            sourceRepo,
		notebookAccessService: notebookAccessService,
		sourceService:         sourceservice.New(objectStore),
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
	access, err := h.notebookAccessService.Resolve(ctx, query.NotebookId, pkgcontext.GetUserId(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", query.NotebookId)
	}
	if !access.CanView() {
		return nil, errors.ErrNoRecord.Msgf("notebook not found, notebook_id=%s", query.NotebookId)
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

	details := make([]*sourceentity.SourceDetail, 0, len(sources))
	for _, source := range sources {
		detail, err := h.sourceService.GetSourceDetail(ctx, source)
		if err != nil {
			return nil, errors.WithMessagef(err, "get source detail failed, source_id=%s", source.Id)
		}
		details = append(details, detail)
	}

	return &ListSourcesResult{
		Sources: details,
		Limit:   query.Limit,
		Offset:  query.Offset,
		HasMore: hasMore,
	}, nil
}
