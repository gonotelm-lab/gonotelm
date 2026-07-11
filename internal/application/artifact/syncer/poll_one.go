package syncer

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

func (s *Syncer) PollOne(ctx context.Context, artifactId valobj.Id) {
	ticker := time.NewTicker(s.cfg.PerTaskInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			done, err := s.pollOnce(ctx, artifactId)
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

func (s *Syncer) pollOnce(ctx context.Context, artifactId valobj.Id) (done bool, err error) {
	a, err := s.repo.FindById(ctx, artifactId)
	if err != nil {
		return true, err
	}
	if a.IsTerminal() {
		return true, nil
	}
	info, err := s.flow.Get(ctx, a.FlowTaskId)
	if err != nil {
		return false, err
	}
	newStatus := mapFlowState(info.State)
	if newStatus == a.Status {
		return false, nil
	}
	switch newStatus {
	case artifactentity.StatusCompleted:
		var out generate.WorkerOutput
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
	if err := s.repo.Save(ctx, a); err != nil {
		return false, err
	}
	for _, evt := range a.PullEvents() {
		if err := s.eventBus.Publish(ctx, evt); err != nil {
			slog.WarnContext(ctx, "publish artifact event failed", "artifact_id", a.Id, "err", err)
		}
	}
	return newStatus.IsTerminal(), nil
}
