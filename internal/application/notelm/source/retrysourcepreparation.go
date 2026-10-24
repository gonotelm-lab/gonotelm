package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type RetrySourcePreparationHandler struct {
	*baseHandler
	eventBus eventbus.Publisher
}

func NewRetrySourcePreparationHandler(
	sourceRepo sourcerepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	eventBus eventbus.Publisher,
) *RetrySourcePreparationHandler {
	return &RetrySourcePreparationHandler{
		baseHandler: newBaseHandler(sourceRepo, notebookAccessService),
		eventBus:    eventBus,
	}
}

func (h *RetrySourcePreparationHandler) Handle(ctx context.Context, sourceId valobj.Id) error {
	_, err := withSourceAccess(h.baseHandler, ctx, sourceId, accessManage, h.retryPreparation)
	return err
}

func (h *RetrySourcePreparationHandler) retryPreparation(
	ctx context.Context,
	targetSource *sourceentity.Source,
) (struct{}, error) {
	if err := targetSource.RetryPreparation(); err != nil {
		return struct{}{}, errors.WithMessagef(err, "retry source preparation failed, source_id=%s", targetSource.Id)
	}

	for _, event := range targetSource.PullEvents() {
		if err := h.eventBus.Publish(ctx, event); err != nil {
			return struct{}{}, errors.WithMessagef(err, "notify source preparing failed, source_id=%s", targetSource.Id)
		}
	}

	if err := h.updateSource(ctx, targetSource); err != nil {
		return struct{}{}, errors.WithMessagef(err, "save source failed, source_id=%s", targetSource.Id)
	}

	return struct{}{}, nil
}
