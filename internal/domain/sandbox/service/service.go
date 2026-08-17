package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxerrors "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const sandboxBindingTTL = 60 * time.Minute

func sandboxBindingExpire(spec entity.Spec) time.Duration {
	if spec.TTL > 0 {
		return spec.TTL
	}
	return sandboxBindingTTL
}

func sandboxLockKey(key entity.SandboxKey) string {
	return fmt.Sprintf("gonotelm:sandbox:lock:%s:%s", key.UserId, key.NotebookId.String())
}

type Service struct {
	repo repository.Repository
	mgr  repository.Manager
	lock adapter.DistributedLock
}

func New(repo repository.Repository, mgr repository.Manager, lock adapter.DistributedLock) *Service {
	return &Service{
		repo: repo,
		mgr:  mgr,
		lock: lock,
	}
}

func (s *Service) GetOrCreateSandbox(
	ctx context.Context,
	key entity.SandboxKey,
	spec entity.Spec,
) (entity.Sandbox, error) {
	ttl := sandboxBindingExpire(spec)
	spec.TTL = ttl

	// 快路径：无锁命中可复用沙箱
	if sb, ok, err := s.tryGetAlive(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return sb, nil
	}

	lockKey := sandboxLockKey(key)
	if err := s.lock.Lock(ctx, lockKey); err != nil {
		return nil, pkgerr.WithMessagef(err, "sandbox get-or-create lock failed: %s", key)
	}
	defer func() {
		if err := s.lock.Unlock(ctx, lockKey); err != nil {
			slog.WarnContext(ctx, "sandbox get-or-create unlock failed",
				slog.Any("err", err),
				slog.String("sandbox_key", key.String()),
			)
		}
	}()

	// 锁内 double-check，避免并发重复创建
	if sb, ok, err := s.tryGetAlive(ctx, key); err != nil {
		return nil, err
	} else if ok {
		return sb, nil
	}

	sb, err := s.mgr.CreateSandbox(ctx, key, spec)
	if err != nil {
		return nil, pkgerr.WithMessagef(err, "mgr create sandbox failed: %s", key)
	}

	if err := s.repo.SetSandbox(ctx, key, sb.Description(), ttl); err != nil {
		slog.WarnContext(ctx, "set sandbox binding failed, rolling back created sandbox",
			slog.Any("err", err.Error()),
			slog.String("sandbox_key", key.String()),
			slog.String("sandbox_id", sb.Id()),
		)
		if delErr := s.mgr.DeleteSandbox(ctx, sb.Id()); delErr != nil {
			slog.WarnContext(ctx, "rollback delete sandbox failed",
				slog.Any("err", delErr),
				slog.String("sandbox_id", sb.Id()),
			)
		}
		return nil, pkgerr.WithMessagef(err, "set sandbox binding failed: %s", key)
	}

	return sb, nil
}

func (s *Service) tryGetAlive(ctx context.Context, key entity.SandboxKey) (entity.Sandbox, bool, error) {
	desc, err := s.repo.GetSandbox(ctx, key)
	if err != nil {
		if pkgerr.Is(err, sandboxerrors.ErrSandboxNotFound) {
			return nil, false, nil
		}
		return nil, false, pkgerr.WithMessagef(err, "repo get sandbox failed: %s", key)
	}
	if desc.Id == "" {
		return nil, false, nil
	}

	sb, err := s.mgr.GetSandbox(ctx, desc.Id)
	if err == nil {
		return sb, true, nil
	}

	slog.WarnContext(ctx, "cached sandbox no longer exists, will recreate",
		slog.Any("err", err),
		slog.String("sandbox_key", key.String()),
		slog.String("sandbox_id", desc.Id),
	)
	// 清掉失效绑定，避免快路径 miss 后锁内 double-check 再连一次死沙箱
	if delErr := s.repo.DeleteSandbox(ctx, key); delErr != nil {
		slog.WarnContext(ctx, "delete stale sandbox binding failed",
			slog.Any("err", delErr),
			slog.String("sandbox_key", key.String()),
		)
	}
	return nil, false, nil
}

func (s *Service) DeleteSandbox(ctx context.Context, key entity.SandboxKey) error {
	desc, err := s.repo.GetSandbox(ctx, key)
	if err != nil {
		if pkgerr.Is(err, sandboxerrors.ErrSandboxNotFound) {
			return nil
		}
		return pkgerr.WithMessagef(err, "repo get sandbox failed: %s", key)
	}

	if err := s.mgr.DeleteSandbox(ctx, desc.Id); err != nil {
		return pkgerr.WithMessagef(err, "mgr delete sandbox failed: %s", key)
	}

	if err := s.repo.DeleteSandbox(ctx, key); err != nil {
		return pkgerr.WithMessagef(err, "repo delete sandbox failed: %s", key)
	}

	slog.InfoContext(ctx, "sandbox deleted",
		slog.String("sandbox_key", key.String()),
		slog.String("sandbox_id", desc.Id),
	)

	return nil
}
