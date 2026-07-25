package artifact

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const retryCancelWaitTimeout = 30 * time.Second
const retryCancelPollInterval = 500 * time.Millisecond

func isTaskTerminal(state flowschema.TaskState) bool {
	switch state {
	case flowschema.TaskState_DONE, flowschema.TaskState_FAILED, flowschema.TaskState_CANCELLED:
		return true
	}
	return false
}

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

	if oldFlowTaskId != "" {
		if err := h.cancelAndWait(ctx, oldFlowTaskId); err != nil {
			slog.WarnContext(ctx, "cancel old flow task on retry failed, proceeding anyway",
				"artifact_id", cmd,
				"old_flow_task_id", oldFlowTaskId,
				slog.Any("err", err),
			)
		}
	}

	payloadBytes, err := sonic.Marshal(a.Payload)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal payload on retry err=%v", err)
	}

	workerInput := buildWorkerInput(a, payloadBytes)
	workerInputBytes, err := sonic.Marshal(workerInput)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal worker input on retry err=%v", err)
	}

	newFlowTaskId, err := h.flowc.Submit(ctx, taskTypeFor(a.Kind), workerInputBytes)
	if err != nil {
		return errors.WithMessage(err, "submit retry task to flow failed")
	}

	if err := a.Retry(newFlowTaskId); err != nil {
		return err
	}
	if err := h.repo.Save(ctx, a); err != nil {
		return errors.WithMessage(err, "save retried artifact failed")
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

func (h *RetryArtifactHandler) cancelAndWait(ctx context.Context, flowTaskId string) error {
	info, err := h.flowc.Get(ctx, flowTaskId)
	if err != nil {
		return nil
	}
	if isTaskTerminal(info.State) {
		return nil
	}

	if err := h.flowc.Cancel(ctx, flowTaskId); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, retryCancelWaitTimeout)
	defer cancel()

	ticker := time.NewTicker(retryCancelPollInterval)
	defer ticker.Stop()

	for {
		info, err := h.flowc.Get(ctx, flowTaskId)
		if err == nil && isTaskTerminal(info.State) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
