package worker

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/contract"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// 查询失败按"产物存在"处理，避免误判
func artifactGone(ctx context.Context, deps *types.WorkerDeps, artifactId valobj.Id) bool {
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

func discardStorageResult(ctx context.Context, deps *types.WorkerDeps, resp *types.Response) {
	if resp == nil || !resp.ResultKind.Storage() || len(resp.Result) == 0 {
		return
	}

	sr, ok := contract.ParseStorageResult(resp.Result)
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
