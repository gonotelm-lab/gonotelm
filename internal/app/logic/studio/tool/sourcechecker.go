package tool

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type SourceChecker interface {
	// 检查agent是否有权限访问source
	CheckPermission(ctx context.Context, sourceId uuid.UUID) error
}

type SourceCheckerFn func(ctx context.Context, sourceId uuid.UUID) error

var _ SourceChecker = SourceCheckerFn(nil)

func (f SourceCheckerFn) CheckPermission(ctx context.Context, sourceId uuid.UUID) error {
	return f(ctx, sourceId)
}
