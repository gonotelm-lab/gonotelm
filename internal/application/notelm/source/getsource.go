package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
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
	objectStore adapter.ObjectStore,
) *GetSourceHandler {
	return &GetSourceHandler{
		baseHandler:   newBaseHandler(sourceRepo),
		sourceService: sourceservice.New(objectStore),
	}
}

func (h *GetSourceHandler) Handle(
	ctx context.Context,
	sourceId valobj.Id,
) (*sourceentity.SourceDetail, error) {
	targetSource, err := h.handle(ctx, sourceId)
	if err != nil {
		return nil, err
	}

	if !targetSource.Status.IsReady() {
		return nil, errors.ErrParams.Msgf("source is not ready, status=%s", targetSource.Status)
	}

	result, err := h.sourceService.GetSourceDetail(ctx, targetSource)
	if err != nil {
		return nil, errors.WithMessagef(err, "get source detail failed, source_id=%s", sourceId)
	}

	return result, nil
}
