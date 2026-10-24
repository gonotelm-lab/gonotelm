package artifact

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CancelArtifactHandler struct {
	*baseHandler
	flowc    flow.TaskClient
	eventBus eventbus.Publisher
}

func NewCancelArtifactHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	flowc flow.TaskClient,
	eventBus eventbus.Publisher,
) *CancelArtifactHandler {
	return &CancelArtifactHandler{baseHandler: newBaseHandler(repo, notebookAccessService), flowc: flowc, eventBus: eventBus}
}

func (h *CancelArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	_, err := withArtifactAccess(h.baseHandler, ctx, cmd, accessManage, h.cancelArtifact)
	return err
}

func (h *CancelArtifactHandler) cancelArtifact(
	ctx context.Context,
	artifact *artifactentity.Artifact,
) (struct{}, error) {
	flowTaskId := artifact.FlowTaskId
	if err := artifact.Cancel(); err != nil {
		return struct{}{}, err
	}
	if err := h.flowc.Cancel(ctx, flowTaskId); err != nil {
		return struct{}{}, errors.WithMessage(err, "cancel flow task failed")
	}
	if err := h.updateArtifact(ctx, artifact); err != nil {
		return struct{}{}, errors.WithMessage(err, "update artifact failed")
	}
	for _, evt := range artifact.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish artifact event failed", "artifact_id", artifact.Id, "err", err)
		}
	}
	return struct{}{}, nil
}
