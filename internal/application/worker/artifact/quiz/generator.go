package quiz

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/bytedance/sonic"
)

const dataKeyResult = "quiz.result"

// Generator 用 pipeline 编排测验题产物。
type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	data := types.NewPipelineData(req)

	p := pipeline.New("worker.quiz")
	p.AddSteps(&quizStep{deps: g.deps})
	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	expect := pipeline.Get[*quizExpectation](data, dataKeyResult)
	resultBytes, err := sonic.Marshal(expect.Quiz)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal quiz content failed, err=%v", err)
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     resultBytes,
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}
