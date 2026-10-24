package artifact

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const (
	retryCancelWaitTimeout  = 30 * time.Second
	retryCancelPollInterval = 500 * time.Millisecond
)

func isTaskTerminal(state flow.TaskState) bool {
	switch state {
	case flow.TaskStateDone, flow.TaskStateFailed, flow.TaskStateCancelled:
		return true
	}
	return false
}

type RetryArtifactHandler struct {
	*baseHandler
	flowc    flow.TaskClient
	poller   Poller
	eventBus eventbus.Publisher
}

type retryArtifactResult struct{}

func NewRetryArtifactHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	flowc flow.TaskClient,
	poller Poller,
	eventBus eventbus.Publisher,
) *RetryArtifactHandler {
	return &RetryArtifactHandler{
		baseHandler: newBaseHandler(repo, notebookAccessService),
		flowc:       flowc,
		poller:      poller,
		eventBus:    eventBus,
	}
}

func (h *RetryArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	_, err := withArtifactAccess(h.baseHandler, ctx, cmd, accessManage, h.retryArtifact)
	return err
}

func (h *RetryArtifactHandler) retryArtifact(
	ctx context.Context,
	artifact *artifactentity.Artifact,
) (*retryArtifactResult, error) {
	if artifact.Status.Completed() {
		return nil, artifacterrors.ErrCannotRetryCompleted
	}

	// 保存为笔记的 artifact 不需要重试
	if artifact.Kind == artifactentity.KindNote {
		return nil, artifacterrors.ErrCannotRetryNote
	}

	oldFlowTaskId := artifact.FlowTaskId

	if oldFlowTaskId != "" {
		if err := h.cancelAndWait(ctx, oldFlowTaskId); err != nil {
			slog.WarnContext(ctx, "cancel old flow task on retry failed, proceeding anyway",
				"artifact_id", artifact.Id,
				"old_flow_task_id", oldFlowTaskId,
				slog.Any("err", err),
			)
		}
	}

	payloadBytes, err := sonic.Marshal(artifact.Payload)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal payload on retry err=%v", err)
	}

	workerInput := buildWorkerInput(artifact, payloadBytes)
	workerInputBytes, err := sonic.Marshal(workerInput)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal worker input on retry err=%v", err)
	}

	newFlowTaskId, err := h.flowc.Submit(ctx, taskTypeFor(artifact.Kind), workerInputBytes)
	if err != nil {
		return nil, errors.WithMessage(err, "submit retry task to flow failed")
	}

	if err := artifact.Retry(newFlowTaskId); err != nil {
		return nil, err
	}
	if err := h.updateArtifact(ctx, artifact); err != nil {
		return nil, errors.WithMessage(err, "update retried artifact failed")
	}

	for _, evt := range artifact.PullEvents() {
		if err := h.eventBus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish artifact event failed", "artifact_id", artifact.Id, "err", err)
		}
	}

	if h.poller != nil {
		go h.poller.PollOne(context.WithoutCancel(ctx), artifact.Id, artifact.FlowTaskId)
	}

	return &retryArtifactResult{}, nil
}

func (h *RetryArtifactHandler) cancelAndWait(ctx context.Context, flowTaskId string) error {
	info, err := h.flowc.Get(ctx, flowTaskId)
	if err != nil || info == nil {
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
		if err == nil && info != nil && isTaskTerminal(info.State) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
