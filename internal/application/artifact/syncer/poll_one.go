package syncer

import (
	"context"
	"log/slog"
	"time"

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
		if err := s.repo.UpdateStatus(ctx, a.Id, newStatus, info.Result, artifactentity.ResultKindInline, a.Title); err != nil {
			return false, err
		}
	case artifactentity.StatusFailed, artifactentity.StatusCancelled:
		if err := s.repo.UpdateStatus(ctx, a.Id, newStatus, nil, "", ""); err != nil {
			return false, err
		}
	case artifactentity.StatusRunning:
		if err := s.repo.UpdateStatus(ctx, a.Id, newStatus, nil, "", ""); err != nil {
			return false, err
		}
	case artifactentity.StatusPending:
		return false, nil
	}
	return newStatus.IsTerminal(), nil
}
