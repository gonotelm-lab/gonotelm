package slides

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
)

// loadSourcesStep 拉取 source 摘要写入 pipeline.Data。
type loadSourcesStep struct {
	deps *types.WorkerDeps
}

func (s *loadSourcesStep) Name() string { return "load-sources" }

func (s *loadSourcesStep) Execute(ctx context.Context, data *pipeline.Data) error {
	sources, err := loadOutlineSources(ctx, s.deps, types.RequestFrom(data).SourceIds)
	if err != nil {
		return errors.WithMessage(err, "load outline sources failed")
	}
	data.Set(dataKeySources, sources)
	return nil
}
