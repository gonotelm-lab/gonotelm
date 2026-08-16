package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxerrors "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/repository"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const sandboxBindingTTL = 12 * time.Hour

func sandboxBindingExpire(spec entity.Spec) time.Duration {
	if spec.TTL > 0 {
		return spec.TTL
	}
	return sandboxBindingTTL
}

type Service struct {
	repo repository.Repository
	mgr  repository.Manager
}

func New(repo repository.Repository, mgr repository.Manager) *Service {
	return &Service{
		repo: repo,
		mgr:  mgr,
	}
}

func (s *Service) GetOrCreateSandbox(
	ctx context.Context,
	key entity.SandboxKey,
	spec entity.Spec,
) (entity.Sandbox, error) {
	desc, err := s.repo.GetSandbox(ctx, key)
	if err != nil && !pkgerr.Is(err, sandboxerrors.ErrSandboxNotFound) {
		return nil, pkgerr.WithMessagef(err, "repo get sandbox failed: %s", key)
	}

	if desc.Id != "" {
		sb, err := s.mgr.GetSandbox(ctx, desc.Id)
		if err == nil {
			return sb, nil
		}

		// 缓存中的 sandbox 已失效，重建
		slog.WarnContext(ctx, "cached sandbox no longer exists, recreating",
			slog.Any("err", err),
			slog.String("sandbox_key", key.String()),
			slog.String("sandbox_id", desc.Id),
		)
	}

	sb, err := s.mgr.CreateSandbox(ctx, key, spec)
	if err != nil {
		return nil, pkgerr.WithMessagef(err, "mgr create sandbox failed: %s", key)
	}

	if err := s.repo.SetSandbox(ctx, key, sb.Description(), sandboxBindingExpire(spec)); err != nil {
		slog.WarnContext(ctx, "set sandbox binding failed",
			slog.Any("err", err.Error()),
			slog.String("sandbox_key", key.String()),
			slog.String("sandbox_id", sb.Id()),
		)
	}

	return sb, nil
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
