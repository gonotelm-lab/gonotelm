package source

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type RetrySourcePreparationHandler struct {
	sourceRepo sourcerepo.Repository
	eventBus   eventbus.EventBus
}

func NewRetrySourcePreparationHandler(
	sourceRepo sourcerepo.Repository,
	eventBus eventbus.EventBus,
) *RetrySourcePreparationHandler {
	return &RetrySourcePreparationHandler{
		sourceRepo: sourceRepo,
		eventBus:   eventBus,
	}
}

func (h *RetrySourcePreparationHandler) Handle(ctx context.Context, sourceId valobj.Id) error {
	targetSource, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return errors.WithMessagef(err, "find source failed, source_id=%s", sourceId)
	}

	err = targetSource.RetryPreparation()
	if err != nil {
		return errors.WithMessagef(err, "retry source preparation failed, source_id=%s", sourceId)
	}

	err = h.sourceRepo.Save(ctx, targetSource)
	if err != nil {
		return errors.WithMessagef(err, "save source failed, source_id=%s", sourceId)
	}

	events := targetSource.PullEvents()
	for _, event := range events {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			targetSource.MarkFailed()
			if rollbackErr := h.sourceRepo.Save(ctx, targetSource); rollbackErr != nil {
				slog.WarnContext(ctx, "rollback source status after retry notify failure failed",
					slog.String("source_id", sourceId.String()),
					slog.Any("err", rollbackErr),
				)
			}
			return errors.WithMessagef(err, "notify source preparing failed, source_id=%s", sourceId)
		}
	}

	return nil
}
