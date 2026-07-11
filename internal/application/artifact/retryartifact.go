package artifact

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type RetryArtifactHandler struct {
	repo     artifactrepo.Repository
	flowc    flow.TaskClient
	poller   Poller
	eventBus eventbus.EventBus
}

func NewRetryArtifactHandler(repo artifactrepo.Repository, flowc flow.TaskClient, poller Poller, eventBus eventbus.EventBus) *RetryArtifactHandler {
	return &RetryArtifactHandler{repo: repo, flowc: flowc, poller: poller, eventBus: eventBus}
}

func (h *RetryArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.repo.FindById(ctx, cmd)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}

	oldFlowTaskId := a.FlowTaskId
	payloadBytes, err := sonic.Marshal(a.Payload)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal payload on retry err=%v", err)
	}
	newFlowTaskId, err := h.flowc.Submit(ctx, taskTypeFor(a.Kind), payloadBytes)
	if err != nil {
		return errors.WithMessage(err, "submit retry task to flow failed")
	}

	if err := a.Retry(newFlowTaskId); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, a); err != nil {
		return errors.WithMessage(err, "save retried artifact failed")
	}

	if oldFlowTaskId != "" && oldFlowTaskId != newFlowTaskId {
		go func() { _ = h.flowc.Cancel(context.WithoutCancel(ctx), oldFlowTaskId) }()
	}
	for _, evt := range a.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish artifact event failed", "artifact_id", a.Id, "err", err)
		}
	}
	if h.poller != nil {
		go h.poller.PollOne(context.WithoutCancel(ctx), a.Id)
	}
	return nil
}
