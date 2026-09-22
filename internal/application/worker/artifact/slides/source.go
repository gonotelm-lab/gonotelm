package slides

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

// loadOutlineSources 拉取每个 source 的摘要，作为大纲与 PPTX 步骤的共享输入。
func loadOutlineSources(ctx context.Context, deps *types.WorkerDeps, sourceIds []valobj.Id) ([]OutlineSource, error) {
	sources := make([]OutlineSource, 0, len(sourceIds))
	for _, id := range sourceIds {
		entry := OutlineSource{Id: id.String()}
		stat, err := deps.Agentize.StatSource(ctx, id)
		if err != nil {
			slog.WarnContext(ctx, "slides outline load source abstract failed",
				slog.String("source_id", id.String()), slog.Any("err", err))
		} else if stat != nil {
			entry.Abstract = stat.Abstract
		}
		sources = append(sources, entry)
	}
	return sources, nil
}
