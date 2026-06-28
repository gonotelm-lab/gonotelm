package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domain "github.com/gonotelm-lab/gonotelm/internal/domain/source"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteSourceHandler struct {
	sourceRepo domain.Repository
	eventBus   eventbus.EventBus
}

func NewDeleteSourceHandler(sourceRepo domain.Repository, eventBus eventbus.EventBus) *DeleteSourceHandler {
	return &DeleteSourceHandler{
		sourceRepo: sourceRepo,
		eventBus:   eventBus,
	}
}

func (h *DeleteSourceHandler) Handle(ctx context.Context, id valobj.Id) error {
	targetSource, err := h.sourceRepo.FindById(ctx, id)
	if err != nil {
		if errors.Is(err, domain.ErrSourceNotFound) {
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
			return errors.WithMessagef(err, "publish source deleted event failed, source_id=%s", id)
		}
	}

	return nil
}
