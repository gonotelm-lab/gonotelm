package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookdomain "github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
)

const notebookSourcesPageSize = 50

type DeleteNotebookSourcesHandler struct {
	sourceRepo        sourcerepo.Repository
	sourceDocRepo     sourcerepo.SourceDocRepository
	sourceStorageRepo sourcerepo.StorageRepository
}

func NewDeleteNotebookSourcesHandler(
	sourceRepo sourcerepo.Repository,
	sourceDocRepo sourcerepo.SourceDocRepository,
	sourceStorageRepo sourcerepo.StorageRepository,
) *DeleteNotebookSourcesHandler {
	return &DeleteNotebookSourcesHandler{
		sourceRepo:        sourceRepo,
		sourceDocRepo:     sourceDocRepo,
		sourceStorageRepo: sourceStorageRepo,
	}
}

func (h *DeleteNotebookSourcesHandler) Handle(
	ctx context.Context,
	evt *notebookdomain.Event,
) error {
	if evt.Action() != notebookdomain.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	sourceIds, objectKeys, err := h.collectNotebookSources(ctx, notebookId)
	if err != nil {
		return err
	}

	if len(sourceIds) > 0 {
		if err := h.sourceDocRepo.BatchDeleteBySourceId(ctx, notebookId, sourceIds); err != nil {
			return errors.WithMessagef(err, "delete source docs failed, notebook_id=%s", notebookId)
		}

		if err := h.sourceRepo.BatchDeleteByIds(ctx, sourceIds); err != nil {
			return errors.WithMessagef(err, "batch delete sources failed, notebook_id=%s", notebookId)
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
		return errors.WithMessagef(err, "delete sources by notebook failed, notebook_id=%s", notebookId)
	}

	slog.InfoContext(ctx, "cleaned up sources for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("source_count", len(sourceIds)),
	)

	return nil
}

func (h *DeleteNotebookSourcesHandler) collectNotebookSources(
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

func RegisterNotebookDeletedConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *DeleteNotebookSourcesHandler,
) error {
	return eventbus.SubscribeNotebookDeleted(ctx, bus, handler.Handle)
}
