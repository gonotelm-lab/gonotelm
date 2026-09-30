package mindmap

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const MindmapMaxOnceToken = 32_000

const dataKeyResult = "mindmap.result"

// Generator 用 pipeline 编排思维导图产物。
type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	data := types.NewPipelineData(req)

	p := pipeline.New("worker.mindmap")
	p.AddSteps(&mindmapStep{deps: g.deps})
	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	expect := pipeline.Get[*mindmapExpectation](data, dataKeyResult)
	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Mindmap),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}
