package flashcard

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

const dataKeyResult = "flashcard.result"

// Generator 用 pipeline 编排闪卡产物。
type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	data := types.NewPipelineData(req)

	p := pipeline.New("worker.flashcard")
	p.AddSteps(&flashcardStep{deps: g.deps})
	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	expect := pipeline.Get[*flashcardExpectation](data, dataKeyResult)
	resultBytes, err := sonic.Marshal(expect.Flashcard)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal flashcard content failed, err=%v", err)
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     resultBytes,
		ResultKind: entity.ResultKindInline,
	}, nil
}
