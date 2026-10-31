package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const notebookSourcesPageSize = 50

// OnNotebookEventHandler 只清 source 自己的资源。
type OnNotebookEventHandler struct {
	sourceRepo sourcerepo.Repository
	cleaner    *cleanup.Cleaner
}

func NewOnNotebookEventHandler(
	sourceRepo sourcerepo.Repository,
	cleaner *cleanup.Cleaner,
) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{sourceRepo: sourceRepo, cleaner: cleaner}
}

func (h *OnNotebookEventHandler) Handle(
	ctx context.Context,
	evt *notebookevent.Event,
) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	sources, err := h.collectNotebookSources(ctx, notebookId)
	if err != nil {
		return errors.WithStack(err)
	}
	if len(sources) == 0 {
		return nil
	}

	if err := h.cleaner.SweepDeletedNotebookSources(ctx, notebookId, sources); err != nil {
		slog.ErrorContext(ctx, "failed to clean up sources for deleted notebook",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)

		return nil
	}

	slog.InfoContext(ctx, "cleaned up sources for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("source_count", len(sources)),
	)

	return nil
}

func (h *OnNotebookEventHandler) collectNotebookSources(
	ctx context.Context,
	notebookId valobj.Id,
) ([]*sourceentity.Source, error) {
	sources := make([]*sourceentity.Source, 0, notebookSourcesPageSize)

	for offset := 0; ; offset += notebookSourcesPageSize {
		page, err := h.sourceRepo.PageListByNotebookId(ctx, notebookId, &sourcerepo.PageListSpec{
			Limit:  notebookSourcesPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, errors.WithMessagef(err, "list sources failed, notebook_id=%s", notebookId)
		}
		if len(page) == 0 {
			break
		}

		sources = append(sources, page...)

		if len(page) < notebookSourcesPageSize {
			break
		}
	}

	return sources, nil
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
