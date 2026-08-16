package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

type Repository interface {
	GetSandbox(ctx context.Context, key entity.SandboxKey) (entity.SandboxDescription, error)
	DeleteSandbox(ctx context.Context, key entity.SandboxKey) error
	SetSandbox(ctx context.Context, key entity.SandboxKey, desc entity.SandboxDescription, ttl time.Duration) error
}
