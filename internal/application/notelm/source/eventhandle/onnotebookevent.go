package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/notelm/source/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const notebookSourcesPageSize = 50

type OnNotebookEventHandler struct {
	sourceRepo       sourcerepo.Repository
	cleanupScheduler *cleanup.CheckpointScheduler
	cleanupSweeper   *cleanup.Sweeper
}

func NewOnNotebookEventHandler(
	sourceRepo sourcerepo.Repository,
	cleanupScheduler *cleanup.CheckpointScheduler,
	cleanupSweeper *cleanup.Sweeper,
) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{
		sourceRepo:       sourceRepo,
		cleanupScheduler: cleanupScheduler,
		cleanupSweeper:   cleanupSweeper,
	}
}

func (h *OnNotebookEventHandler) Handle(
	ctx context.Context,
	evt *notebookevent.Event,
) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	refs, err := h.collectNotebookSources(ctx, notebookId)
	if err != nil {
		return errors.WithStack(err)
	}

	// 先注册一个定时任务兜底清理流程
	if err := h.cleanupScheduler.ScheduleNotebookCleanup(ctx, notebookId, refs); err != nil {
		slog.ErrorContext(ctx, "register notebook cleanup checkpoint failed",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
	}

	if err := h.cleanupSweeper.SweepNotebook(ctx, notebookId, refs); err != nil {
		slog.ErrorContext(ctx, "failed to clean up sources for deleted notebook",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
	}

	slog.InfoContext(ctx, "cleaned up sources for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("source_count", len(refs)),
	)

	return nil
}

func (h *OnNotebookEventHandler) collectNotebookSources(
	ctx context.Context,
	notebookId valobj.Id,
) ([]cleanup.SourceRef, error) {
	refs := make([]cleanup.SourceRef, 0, notebookSourcesPageSize)

	for offset := 0; ; offset += notebookSourcesPageSize {
		sources, err := h.sourceRepo.ListByNotebookId(ctx, notebookId, &sourcerepo.ListSpec{
			Limit:  notebookSourcesPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, errors.WithMessagef(err, "list sources failed, notebook_id=%s", notebookId)
		}
		if len(sources) == 0 {
			break
		}

		for _, source := range sources {
			refs = append(refs, cleanup.SourceRef{
				SourceId: source.Id,
				Keys:     cleanup.EncodeKeys(source.ObjectStoreKeys()),
			})
		}

		if len(sources) < notebookSourcesPageSize {
			break
		}
	}

	return refs, nil
}

func RegisterNotebookEventConsumer(
	bus eventbus.InProcessEventBus,
	handler *OnNotebookEventHandler,
) error {
	return bus.Subscribe(notebookevent.TopicNotebookEvent,
		func(ctx context.Context, evt event.Event) error {
			nbEvt, err := eventbus.AssertEvent[*notebookevent.Event](evt)
			if err != nil {
				return err
			}
			return handler.Handle(ctx, nbEvt)
		},
	)
}
