package infographic

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
)

// 步骤之间通过 pipeline.Data 传递的键。
const (
	dataKeyCheckpoint  = "infographic.checkpoint"
	dataKeyExpectation = "infographic.expectation"
	dataKeyResult      = "infographic.result"
)

// Generator 用 pipeline 编排信息图产物：文生图 prompt（field1）→ 生成并存储图片。
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

	p := pipeline.New("worker.infographic")
	p.AddSteps(
		&imagePromptStep{deps: g.deps, checkpoints: g.checkpoints},
		newImageStep(g.deps),
	)

	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(pipeline.Get[*StorageResult](data, dataKeyResult))
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal infographic storage result err=%v", err)
	}

	return &types.Response{
		Title:      pipeline.Get[*infoGraphicExpectation](data, dataKeyExpectation).Title,
		Result:     result,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}

func payloadFrom(req *types.Request) *artifactentity.InfoGraphicPayload {
	return artifactentity.PayloadAs[*artifactentity.InfoGraphicPayload](req.Payload)
}

func checkpointFrom(data *pipeline.Data) *workerentity.Checkpoint {
	return pipeline.Get[*workerentity.Checkpoint](data, dataKeyCheckpoint)
}

func expectationFrom(data *pipeline.Data) *infoGraphicExpectation {
	return pipeline.Get[*infoGraphicExpectation](data, dataKeyExpectation)
}
