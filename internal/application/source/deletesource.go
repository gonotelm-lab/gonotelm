package source

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteSourceHandler struct {
	sourceRepo sourcerepo.Repository
	eventBus   eventbus.EventBus
}

func NewDeleteSourceHandler(sourceRepo sourcerepo.Repository, eventBus eventbus.EventBus) *DeleteSourceHandler {
	return &DeleteSourceHandler{
		sourceRepo: sourceRepo,
		eventBus:   eventBus,
	}
}

func (h *DeleteSourceHandler) Handle(ctx context.Context, id valobj.Id) error {
	targetSource, err := h.sourceRepo.FindById(ctx, id)
	if err != nil {
		if errors.Is(err, domainerr.ErrSourceNotFound) {
			return nil
		}
		return errors.WithMessagef(err, "find source failed, source_id=%s", id)
	}

	targetSource.Delete()
	err = h.sourceRepo.Save(ctx, targetSource)
	if err != nil {
		return errors.WithMessagef(err, "delete source failed, source_id=%s", id)
	}

	// TODO handle source docs and storage cleanup via events
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
