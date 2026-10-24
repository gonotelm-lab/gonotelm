package source

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
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
	notebookAccessService *notebookservice.AccessService,
	eventBus eventbus.Publisher,
	cleaner *cleanup.Cleaner,
) *DeleteSourceHandler {
	return &DeleteSourceHandler{
		baseHandler: newBaseHandler(sourceRepo, notebookAccessService),
		eventBus:    eventBus,
		cleaner:     cleaner,
	}
}

func (h *DeleteSourceHandler) Handle(ctx context.Context, id valobj.Id) error {
	_, err := withSourceAccess(h.baseHandler, ctx, id, accessManage, h.deleteSource)
	if errors.Is(err, domainerr.ErrSourceNotFound) {
		return nil
	}
	return err
}

func (h *DeleteSourceHandler) deleteSource(
	ctx context.Context,
	targetSource *sourceentity.Source,
) (struct{}, error) {
	id := targetSource.Id

	targetSource.Delete()
	if err := h.sourceRepo.DeleteById(ctx, targetSource.Id); err != nil {
		return struct{}{}, errors.WithMessagef(err, "delete source failed, source_id=%s", id)
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
		if err := h.eventBus.Publish(ctx, event); err != nil {
			slog.ErrorContext(ctx, "publish source deleted event failed",
				slog.String("source_id", id.String()),
				slog.Any("err", err),
			)
		}
	}

	return struct{}{}, nil
}
