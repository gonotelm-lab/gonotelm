package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	sourceservice "github.com/gonotelm-lab/gonotelm/internal/domain/source/service/source"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetSourceHandler struct {
	*baseHandler
	sourceService sourceservice.Service
}

func NewGetSourceHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	objectStore adapter.ObjectStore,
) *GetSourceHandler {
	return &GetSourceHandler{
		baseHandler:   newBaseHandler(sourceRepo, notebookAccessService),
		sourceService: sourceservice.New(objectStore),
	}
}

func (h *GetSourceHandler) Handle(
	ctx context.Context,
	sourceId valobj.Id,
) (*sourceentity.SourceDetail, error) {
	return withSourceAccess(h.baseHandler, ctx, sourceId, accessView, h.getSource)
}

func (h *GetSourceHandler) getSource(
	ctx context.Context,
	targetSource *sourceentity.Source,
) (*sourceentity.SourceDetail, error) {
	if !targetSource.Status.IsReady() {
		return nil, errors.ErrParams.Msgf("source is not ready, status=%s", targetSource.Status)
	}

	result, err := h.sourceService.GetSourceDetail(ctx, targetSource)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source detail failed, source_id=%s", targetSource.Id)
	}

	return result, nil
}
