package tool

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func permissionDeniedForSource(sourceId uuid.UUID) error {
	return fmt.Errorf("source access denied for source_id=%s", sourceId)
}

type SourceChecker interface {
	// 检查agent是否有权限访问source
	CheckPermission(ctx context.Context, sourceId uuid.UUID) error
}

type SourceCheckerFn func(ctx context.Context, sourceId uuid.UUID) error

var _ SourceChecker = SourceCheckerFn(nil)

func (f SourceCheckerFn) CheckPermission(ctx context.Context, sourceId uuid.UUID) error {
	return f(ctx, sourceId)
}
