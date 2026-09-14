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
	return fmt.Sprintf("worker:sandbox:lock:%s:%s", key.UserId.String(), key.NotebookId.String())
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
		s.touchSandbox(ctx, key, sb, ttl)
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
		s.touchSandbox(ctx, key, sb, ttl)
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

// touchSandbox 复用沙箱时刷新沙箱过期时间，成功后再刷新 Redis 绑定 TTL（best-effort，失败只告警）。
// 沙箱续期失败时不刷新 Redis TTL，避免沙箱已失效但绑定被无限续期。
func (s *Service) touchSandbox(ctx context.Context, key entity.SandboxKey, sb entity.Sandbox, ttl time.Duration) {
	if err := s.mgr.RenewSandbox(ctx, sb.Id(), ttl); err != nil {
		slog.WarnContext(ctx, "renew sandbox expiration failed, skip refreshing binding ttl",
			slog.Any("err", err),
			slog.String("sandbox_key", key.String()),
			slog.String("sandbox_id", sb.Id()),
		)
		return
	}

	if err := s.repo.SetSandbox(ctx, key, sb.Description(), ttl); err != nil {
		slog.WarnContext(ctx, "refresh sandbox binding failed",
			slog.Any("err", err),
			slog.String("sandbox_key", key.String()),
			slog.String("sandbox_id", sb.Id()),
		)
	}
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
		// 命中句柄不代表沙箱还活着：复用前做存活检查，失败则视为失效
		if pingErr := sb.Ping(ctx); pingErr == nil {
			return sb, true, nil
		} else {
			err = pingErr
			// 句柄已失效：主动从 manager 内存缓存驱逐，避免后续继续复用
			if evictErr := s.mgr.EvictSandbox(ctx, desc.Id); evictErr != nil {
				slog.WarnContext(ctx, "evict dead sandbox handle failed",
					slog.Any("err", evictErr),
					slog.String("sandbox_key", key.String()),
					slog.String("sandbox_id", desc.Id),
				)
			}
		}
	}

	slog.WarnContext(ctx, "cached sandbox no longer alive, will recreate",
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
