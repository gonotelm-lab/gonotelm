package slides

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
)

// 步骤之间通过 pipeline.Data 传递的键。
const (
	dataKeyCheckpoint = "slides.checkpoint"
	dataKeySources    = "slides.sources"
	dataKeyOutline    = "slides.outline"
	dataKeySandbox    = "slides.sandbox"
	dataKeyResult     = "slides.result"
)

// Generator 用 pipeline 编排幻灯片产物：加载 source → 并发生成大纲与准备沙箱 → 生成并上传 PPTX。
type Generator struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{
		deps:        deps,
		checkpoints: types.NewCheckpointStore(deps.CheckpointRepository),
	}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	data := types.NewPipelineData(req)
	data.Set(dataKeyCheckpoint, g.checkpoints.Load(ctx, req.ArtifactId))

	p := pipeline.New("worker.slides")
	p.AddSteps(
		&loadSourcesStep{deps: g.deps},
		// 大纲生成与沙箱准备互不依赖，并行执行；任一失败即中止整个产物生成。
		pipeline.NewParallelStep("prepare", true,
			&outlineStep{deps: g.deps, checkpoints: g.checkpoints},
			&sandboxStep{deps: g.deps},
		),
		&pptxStep{deps: g.deps},
	)

	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	resultBytes, err := sonic.Marshal(pipeline.Get[*SlidesStorageResult](data, dataKeyResult))
	if err != nil {
		return nil, errors.Wrap(err, "marshal slides generation result failed")
	}

	return &types.Response{
		Title:      outlineFrom(data).Title,
		Result:     resultBytes,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}

func checkpointFrom(data *pipeline.Data) *workerentity.Checkpoint {
	return pipeline.Get[*workerentity.Checkpoint](data, dataKeyCheckpoint)
}

func sourcesFrom(data *pipeline.Data) []OutlineSource {
	return pipeline.Get[[]OutlineSource](data, dataKeySources)
}

func outlineFrom(data *pipeline.Data) *slidesOutlineExpectation {
	return pipeline.Get[*slidesOutlineExpectation](data, dataKeyOutline)
}

func sandboxFrom(data *pipeline.Data) sandboxent.Sandbox {
	return pipeline.Get[sandboxent.Sandbox](data, dataKeySandbox)
}
