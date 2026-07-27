package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceevent "github.com/gonotelm-lab/gonotelm/internal/domain/source/event"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CleanupDeletedSourceHandler struct {
	sourceDocRepo     sourcerepo.SourceDocRepository
	sourceStorageRepo sourcerepo.StorageRepository
}

func NewCleanupDeletedSourceHandler(
	sourceDocRepo sourcerepo.SourceDocRepository,
	sourceStorageRepo sourcerepo.StorageRepository,
) *CleanupDeletedSourceHandler {
	return &CleanupDeletedSourceHandler{
		sourceDocRepo:     sourceDocRepo,
		sourceStorageRepo: sourceStorageRepo,
	}
}

func (h *CleanupDeletedSourceHandler) Handle(ctx context.Context, evt *sourceevent.DeleteEvent) error {
	sourceId := evt.SourceId()
	notebookId := evt.NotebookId()

	if err := h.sourceDocRepo.BatchDeleteBySourceId(ctx, notebookId, []valobj.Id{sourceId}); err != nil {
		return errors.WithMessagef(err, "delete source docs failed, source_id=%s", sourceId)
	}

	for _, key := range evt.ObjectStoreKeys() {
		if err := h.sourceStorageRepo.DeleteObject(ctx, key); err != nil {
			slog.WarnContext(ctx, "delete source object failed",
				slog.String("source_id", sourceId.String()),
				slog.String("store_key", key),
				slog.Any("err", err),
			)
		}
	}

	slog.InfoContext(ctx, "cleaned up deleted source",
		slog.String("source_id", sourceId.String()),
		slog.String("notebook_id", notebookId.String()),
	)

	return nil
}

func RegisterSourceDeletedConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *CleanupDeletedSourceHandler,
) error {
	composite, err := eventbus.AsComposite(bus)
	if err != nil {
		return err
	}

	return composite.SubscribeInner(ctx, sourceevent.DeleteTopic,
		func(ctx context.Context, evt event.Event) error {
			delEvt, err := eventbus.AssertEvent[*sourceevent.DeleteEvent](evt)
			if err != nil {
				return err
			}
			return handler.Handle(ctx, delEvt)
		},
	)
}
