package source

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteSourceHandler struct {
	*baseHandler
	eventBus eventbus.Publisher
	cleaner  *cleanup.Cleaner
}

func NewDeleteSourceHandler(
	sourceRepo sourcerepo.Repository,
	eventBus eventbus.Publisher,
	cleaner *cleanup.Cleaner,
) *DeleteSourceHandler {
	return &DeleteSourceHandler{
		baseHandler: newBaseHandler(sourceRepo),
		eventBus:    eventBus,
		cleaner:     cleaner,
	}
}

func (h *DeleteSourceHandler) Handle(ctx context.Context, id valobj.Id) error {
	targetSource, err := h.handle(ctx, id)
	if err != nil {
		if errors.Is(err, domainerr.ErrSourceNotFound) {
			return nil
		}
		return err
	}

	targetSource.Delete()
	if err := h.sourceRepo.DeleteById(ctx, targetSource.Id); err != nil {
		return errors.WithMessagef(err, "delete source failed, source_id=%s", id)
	}

	if err := h.cleaner.ScheduleSourceCleanup(
		ctx,
		targetSource.NotebookId,
		targetSource.Id,
		targetSource.ObjectStoreKeys(),
	); err != nil {
		slog.ErrorContext(ctx, "register source cleanup checkpoint failed",
			slog.String("source_id", id.String()),
			slog.Any("err", err),
		)
	}

	for _, event := range targetSource.PullEvents() {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			slog.ErrorContext(ctx, "publish source deleted event failed",
				slog.String("source_id", id.String()),
				slog.Any("err", err),
			)
		}
	}

	return nil
}
