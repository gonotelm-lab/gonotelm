package artifact

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CancelArtifactHandler struct {
	*baseHandler
	flowc    flow.TaskClient
	eventBus eventbus.EventBus
}

func NewCancelArtifactHandler(repo artifactrepo.Repository, flowc flow.TaskClient, eventBus eventbus.EventBus) *CancelArtifactHandler {
	return &CancelArtifactHandler{baseHandler: newBaseHandler(repo), flowc: flowc, eventBus: eventBus}
}

func (h *CancelArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.handle(ctx, cmd)
	if err != nil {
		return err
	}
	flowTaskId := a.FlowTaskId
	if err := a.Cancel(); err != nil {
		return err
	}
	if err := h.flowc.Cancel(ctx, flowTaskId); err != nil {
		return errors.WithMessage(err, "cancel flow task failed")
	}
	if err := h.repo.Save(ctx, a); err != nil {
		return errors.WithMessage(err, "save artifact failed")
	}
	for _, evt := range a.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish artifact event failed", "artifact_id", a.Id, "err", err)
		}
	}
	return nil
}
