package syncer

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	flowschema "github.com/gonotelm-lab/flow/api/schema/v1"
	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func (s *Syncer) PollOne(ctx context.Context, artifactId valobj.Id, flowTaskId string) {
	ticker := time.NewTicker(s.cfg.PerTaskInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			done, err := s.pollOnce(ctx, artifactId, flowTaskId)
			if err != nil {
				slog.WarnContext(ctx, "pollOnce failed", "artifact_id", artifactId, "err", err)
				continue
			}
			if done {
				return
			}
		}
	}
}

func (s *Syncer) pollOnce(ctx context.Context, artifactId valobj.Id, flowTaskId string) (done bool, err error) {
	a, err := s.repo.FindById(ctx, artifactId)
	if err != nil {
		if errors.Is(err, artifacterrors.ErrArtifactNotFound) {
			// 产物记录没了，但 worker 可能还在生成、还会上传对象
			return s.reapOrphan(ctx, flowTaskId)
		}

		return true, err
	}

	if a.Status == artifactentity.StatusCompleted || a.Status == artifactentity.StatusCancelled {
		return true, nil
	}

	info, err := s.flow.Get(ctx, a.FlowTaskId)
	if err != nil {
		return false, err
	}

	newStatus := mapFlowState(info.State)
	if newStatus == a.Status {
		if newStatus == artifactentity.StatusFailed && retriesExhausted(info) {
			return true, nil
		}
		return false, nil
	}

	switch newStatus {
	case artifactentity.StatusCompleted:
		var out contract.WorkerOutput
		if err := sonic.Unmarshal(info.Result, &out); err != nil {
			return false, err
		}
		a.MarkCompleted(out.Result, artifactentity.ResultKind(out.ResultKind), out.Title)
	case artifactentity.StatusFailed:
		a.MarkFailed()
	case artifactentity.StatusCancelled:
		a.MarkCancelled()
	case artifactentity.StatusRunning:
		a.MarkRunning()
	case artifactentity.StatusPending:
		return false, nil
	}

	affected, err := s.repo.UpdateExisting(ctx, a)
	if err != nil {
		return false, err
	}

	if affected == 0 {
		if newStatus == artifactentity.StatusCompleted {
			s.deleteOrphanStorage(ctx, a.FlowTaskId, info.Result)
		}

		slog.InfoContext(ctx, "artifact row already deleted, stop syncing",
			slog.String("artifact_id", a.Id.String()),
		)

		return true, nil
	}

	for _, evt := range a.PullEvents() {
		if err := s.eventBus.Publish(ctx, evt); err != nil {
			slog.WarnContext(ctx, "publish artifact event failed", "artifact_id", a.Id, "err", err)
		}
	}

	switch newStatus {
	case artifactentity.StatusCompleted, artifactentity.StatusCancelled:
		return true, nil
	case artifactentity.StatusFailed:
		return retriesExhausted(info), nil
	}

	return false, nil
}

// reapOrphan 产物记录已删但 flow 任务可能还在跑：盯到终态，删掉已上传的 storage 产物。
func (s *Syncer) reapOrphan(ctx context.Context, flowTaskId string) (bool, error) {
	if flowTaskId == "" {
		return true, nil
	}

	info, err := s.flow.Get(ctx, flowTaskId)
	if err != nil {
		return false, err
	}

	switch info.State {
	case flowschema.TaskState_DONE, flowschema.TaskState_CANCELLED:
		// 被取消的任务也可能已上传完：有 result 就删，没有就是 no-op
		s.deleteOrphanStorage(ctx, flowTaskId, info.Result)

		return true, nil
	case flowschema.TaskState_FAILED:
		return true, nil
	}

	return false, nil
}

func (s *Syncer) deleteOrphanStorage(ctx context.Context, flowTaskId string, result []byte) {
	// 被取消/未产生结果的任务 result 为空，属于预期，不用当失败
	if s.storage == nil || len(result) == 0 {
		return
	}

	var out contract.WorkerOutput
	if err := sonic.Unmarshal(result, &out); err != nil {
		slog.WarnContext(ctx, "unmarshal worker output for orphan cleanup failed",
			slog.String("flow_task_id", flowTaskId),
			slog.Any("err", err),
		)

		return
	}
	if !artifactentity.ResultKind(out.ResultKind).Storage() {
		return
	}

	sr, ok := artifactentity.ParseStorageResult(out.Result)
	if !ok {
		slog.WarnContext(ctx, "orphan artifact result is not a valid storage result, skip",
			slog.String("flow_task_id", flowTaskId),
		)

		return
	}

	if err := s.storage.BatchDeleteObject(ctx, []valobj.StoreKey{sr.StoreKey}); err != nil {
		slog.WarnContext(ctx, "delete orphan artifact object failed",
			slog.String("flow_task_id", flowTaskId),
			slog.String("store_key", sr.StoreKey.String()),
			slog.Any("err", err),
		)
	}
}

func retriesExhausted(info *flow.TaskInfo) bool {
	return info.AttemptNo > info.MaxRetry
}
