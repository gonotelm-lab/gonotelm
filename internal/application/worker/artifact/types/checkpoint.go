package types

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	workererrors "github.com/gonotelm-lab/gonotelm/internal/domain/worker/errors"
	workerrepo "github.com/gonotelm-lab/gonotelm/internal/domain/worker/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// CheckpointStore 封装 artifact checkpoint 的读写，供各产物 Generator 及其步骤生成器共用。
type CheckpointStore struct {
	repo workerrepo.CheckpointRepository
}

func NewCheckpointStore(repo workerrepo.CheckpointRepository) *CheckpointStore {
	return &CheckpointStore{repo: repo}
}

// Load 读取 artifact 的 checkpoint；不存在或读取失败时返回 nil。
func (s *CheckpointStore) Load(ctx context.Context, artifactId valobj.Id) *workerentity.Checkpoint {
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

func (s *CheckpointStore) Save(ctx context.Context, ckpt *workerentity.Checkpoint) error {
	if ckpt == nil {
		return errors.New("checkpoint is nil")
	}
	return s.repo.Save(ctx, ckpt)
}
