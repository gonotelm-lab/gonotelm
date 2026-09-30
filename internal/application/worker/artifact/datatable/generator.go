package datatable

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const dataKeyResult = "datatable.result"

// Generator 用 pipeline 编排数据表产物。
type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	data := types.NewPipelineData(req)

	p := pipeline.New("worker.datatable")
	p.AddSteps(&dataTableStep{deps: g.deps})
	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	expect := pipeline.Get[*dataTableExpectation](data, dataKeyResult)
	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Table),
		ResultKind: entity.ResultKindInline,
	}, nil
}
