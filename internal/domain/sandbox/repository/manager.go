package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

type Manager interface {
	CreateSandbox(ctx context.Context, key entity.SandboxKey, spec entity.Spec) (entity.Sandbox, error)
	GetSandbox(ctx context.Context, sandboxId string) (entity.Sandbox, error)
	// RenewSandbox 把沙箱过期时间延长为「当前时间 + ttl」。
	RenewSandbox(ctx context.Context, sandboxId string, ttl time.Duration) error
	DeleteSandbox(ctx context.Context, sandboxId string) error
	// EvictSandbox 只丢弃本地缓存的沙箱句柄，不触碰远端。用于句柄已失效时清理内存缓存。
	EvictSandbox(ctx context.Context, sandboxId string) error
}
