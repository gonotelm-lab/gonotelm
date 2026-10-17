// Package artifact 汇总各产物 pipeline 共用的守卫逻辑。
package artifact

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// IsDeleted 判断产物记录是否已被删除；查询失败按存在处理。
func IsDeleted(ctx context.Context, deps *types.WorkerDeps, artifactId valobj.Id) bool {
	if deps.ArtifactRepo == nil {
		return false
	}

	if _, err := deps.ArtifactRepo.FindById(ctx, artifactId); err == nil {
		return false
	} else if !errors.Is(err, artifacterrors.ErrArtifactNotFound) {
		slog.WarnContext(ctx, "check artifact exists failed, keep generating",
			slog.String("artifact_id", artifactId.String()),
			slog.Any("err", err),
		)

		return false
	}

	return true
}

// DiscardStorageResult 丢弃无人认领的 storage 产物对象。
func DiscardStorageResult(ctx context.Context, deps *types.WorkerDeps, resp *types.Response) {
	if resp == nil || !resp.ResultKind.Storage() || len(resp.Result) == 0 {
		return
	}

	sr, ok := artifactentity.ParseStorageResult(resp.Result)
	if !ok {
		return
	}

	if err := deps.ObjectStorage.BatchDeleteObject(ctx, []valobj.StoreKey{sr.StoreKey}); err != nil {
		slog.WarnContext(ctx, "discard uploaded artifact object failed",
			slog.String("store_key", sr.StoreKey.String()),
			slog.Any("err", err),
		)
	}
}
