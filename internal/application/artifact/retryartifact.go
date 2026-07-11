package artifact

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type RetryArtifactHandler struct {
	repo   artifactrepo.Repository
	flowc  flow.TaskClient
	poller Poller
}

func NewRetryArtifactHandler(repo artifactrepo.Repository, flowc flow.TaskClient, poller Poller) *RetryArtifactHandler {
	return &RetryArtifactHandler{repo: repo, flowc: flowc, poller: poller}
}

func (h *RetryArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.repo.FindById(ctx, cmd)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if a.Status != artifactentity.StatusFailed && a.Status != artifactentity.StatusCancelled {
		return artifacterrors.ErrCannotRetryInState
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

	if err := h.repo.UpdateFlowTaskId(ctx, a.Id, newFlowTaskId, []artifactentity.Status{artifactentity.StatusFailed, artifactentity.StatusCancelled}); err != nil {
		return errors.WithMessage(err, "update flow task id failed")
	}
	if err := h.repo.UpdateStatus(ctx, a.Id, artifactentity.StatusPending, nil, "", ""); err != nil {
		return errors.WithMessage(err, "save retried artifact failed")
	}

	if oldFlowTaskId != "" && oldFlowTaskId != newFlowTaskId {
		go func() { _ = h.flowc.Cancel(context.WithoutCancel(ctx), oldFlowTaskId) }()
	}
	if h.poller != nil {
		go h.poller.PollOne(context.WithoutCancel(ctx), a.Id)
	}
	return nil
}
