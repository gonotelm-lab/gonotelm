package syncer

import (
	"context"
	"log/slog"
	"time"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
)

func (s *Syncer) globalLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(s.cfg.GlobalInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-ticker.C:
			if err := s.scanOnce(ctx); err != nil {
				slog.WarnContext(ctx, "syncer scan once failed", "err", err)
			}
		}
	}
}

func (s *Syncer) scanOnce(ctx context.Context) error {
	rows, err := s.repo.ListByStatus(ctx, &artifactrepo.ListByStatusSpec{
		Statuses: []artifactentity.Status{artifactentity.StatusPending, artifactentity.StatusRunning},
		Limit:    s.cfg.GlobalBatchSize,
	})
	if err != nil {
		return err
	}
	for _, a := range rows {
		_, _ = s.pollOnce(ctx, a.Id)
	}
	return nil
}
