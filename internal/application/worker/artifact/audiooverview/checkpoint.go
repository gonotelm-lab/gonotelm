package audiooverview

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	workererrors "github.com/gonotelm-lab/gonotelm/internal/domain/worker/errors"
	workerrepo "github.com/gonotelm-lab/gonotelm/internal/domain/worker/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// checkpointStore 封装 checkpoint 读写，供 Generator、各步骤生成器与 audioSynthizer 共用。
type checkpointStore struct {
	repo workerrepo.CheckpointRepository
}

func newCheckpointStore(repo workerrepo.CheckpointRepository) *checkpointStore {
	return &checkpointStore{repo: repo}
}

func (s *checkpointStore) load(ctx context.Context, artifactId valobj.Id) *workerentity.Checkpoint {
	ckpt, err := s.repo.FindByArtifactId(ctx, artifactId)
	if err != nil {
		if !errors.Is(err, workererrors.ErrCheckpointNotFound) {
			slog.ErrorContext(ctx, "find checkpoint failed",
				slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		}
		return nil
	}
	return ckpt
}

func (s *checkpointStore) save(ctx context.Context, ckpt *workerentity.Checkpoint) error {
	if ckpt == nil {
		return errors.New("audio overview checkpoint is nil")
	}
	return s.repo.Save(ctx, ckpt)
}
