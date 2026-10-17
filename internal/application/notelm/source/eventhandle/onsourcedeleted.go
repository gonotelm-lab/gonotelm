package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/source/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	sourceevent "github.com/gonotelm-lab/gonotelm/internal/domain/source/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
)

type OnSourceDeletedEventHandler struct {
	cleanupScheduler *cleanup.CheckpointScheduler
	cleanupSweeper   *cleanup.Sweeper
}

func NewOnSourceDeletedEventHandler(
	cleanupScheduler *cleanup.CheckpointScheduler,
	cleanupSweeper *cleanup.Sweeper,
) *OnSourceDeletedEventHandler {
	return &OnSourceDeletedEventHandler{
		cleanupScheduler: cleanupScheduler,
		cleanupSweeper:   cleanupSweeper,
	}
}

func (h *OnSourceDeletedEventHandler) Handle(ctx context.Context, evt *sourceevent.DeleteEvent) error {
	sourceId := evt.SourceId()
	notebookId := evt.NotebookId()
	objectKeys := cleanup.EncodeKeys(evt.ObjectStoreKeys())

	// 先埋检查点再做立即清理：清理失败、进程退出、事后才落地的对象都靠这一轮兜底。
	if err := h.cleanupScheduler.ScheduleSourceCleanup(ctx, notebookId, sourceId, evt.ObjectStoreKeys()); err != nil {
		slog.ErrorContext(ctx, "register source cleanup checkpoint failed",
			slog.String("source_id", sourceId.String()),
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
	}

	if err := h.cleanupSweeper.SweepSource(ctx, notebookId, sourceId, objectKeys); err != nil {
		slog.ErrorContext(ctx, "failed to clean up deleted source",
			slog.String("source_id", sourceId.String()),
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)

		return err
	}

	slog.InfoContext(ctx, "cleaned up deleted source",
		slog.String("source_id", sourceId.String()),
		slog.String("notebook_id", notebookId.String()),
	)

	return nil
}

func RegisterSourceDeletedConsumer(
	bus eventbus.InProcessEventBus,
	handler *OnSourceDeletedEventHandler,
) error {
	return bus.Subscribe(sourceevent.DeleteTopic,
		func(ctx context.Context, evt event.Event) error {
			delEvt, err := eventbus.AssertEvent[*sourceevent.DeleteEvent](evt)
			if err != nil {
				return err
			}
			return handler.Handle(ctx, delEvt)
		},
	)
}
