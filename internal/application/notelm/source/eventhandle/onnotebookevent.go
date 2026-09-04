package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
)

const notebookSourcesPageSize = 50

type OnNotebookEventHandler struct {
	sourceRepo        sourcerepo.Repository
	sourceDocRepo     sourcerepo.SourceDocRepository
	sourceStorageRepo sourcerepo.StorageRepository
}

func NewOnNotebookEventHandler(
	sourceRepo sourcerepo.Repository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	sourceStorageRepo sourcerepo.StorageRepository,
) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{
		sourceRepo:        sourceRepo,
		sourceDocRepo:     sourceDocRepo,
		sourceStorageRepo: sourceStorageRepo,
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
	sourceIds, objectKeys, err := h.collectNotebookSources(ctx, notebookId)
	if err != nil {
		return errors.WithStack(err)
	}

	if len(sourceIds) > 0 {
		if err := h.sourceDocRepo.BatchDeleteBySourceId(ctx, notebookId, sourceIds); err != nil {
			slog.ErrorContext(ctx, "failed to delete source docs",
				slog.Any("err", err), slog.String("notebook_id", notebookId.String()),
			)
		}

		if err := h.sourceRepo.BatchDeleteByIds(ctx, sourceIds); err != nil {
			slog.ErrorContext(ctx, "failed to batch delete sources",
				slog.Any("err", err), slog.String("notebook_id", notebookId.String()),
			)
		}
	}

	for _, key := range objectKeys {
		if err := h.sourceStorageRepo.DeleteObject(ctx, key); err != nil {
			slog.WarnContext(ctx, "delete source object failed",
				slog.String("notebook_id", notebookId.String()),
				slog.String("store_key", key),
				slog.Any("err", err),
			)
		}
	}

	if err := h.sourceRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		slog.ErrorContext(ctx, "failed to delete sources by notebook",
			slog.Any("err", err), slog.String("notebook_id", notebookId.String()),
		)
	}

	slog.InfoContext(ctx, "cleaned up sources for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("source_count", len(sourceIds)),
	)

	return nil
}

func (h *OnNotebookEventHandler) collectNotebookSources(
	ctx context.Context,
	notebookId valobj.Id,
) ([]valobj.Id, []string, error) {
	sourceIds := make([]valobj.Id, 0, notebookSourcesPageSize)
	objectKeys := make([]string, 0, notebookSourcesPageSize*2)

	for offset := 0; ; offset += notebookSourcesPageSize {
		sources, err := h.sourceRepo.ListByNotebookId(ctx, notebookId, &sourcerepo.ListSpec{
			Limit:  notebookSourcesPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, nil, errors.WithMessagef(err, "list sources failed, notebook_id=%s", notebookId)
		}
		if len(sources) == 0 {
			break
		}

		for _, source := range sources {
			sourceIds = append(sourceIds, source.Id)
			objectKeys = append(objectKeys, source.ObjectStoreKeys()...)
		}

		if len(sources) < notebookSourcesPageSize {
			break
		}
	}

	return sourceIds, slices.Unique(objectKeys), nil
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
