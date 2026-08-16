package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

type Manager interface {
	CreateSandbox(ctx context.Context, key entity.SandboxKey, spec entity.Spec) (entity.Sandbox, error)
	GetSandbox(ctx context.Context, sandboxId string) (entity.Sandbox, error)
	DeleteSandbox(ctx context.Context, sandboxId string) error
}
