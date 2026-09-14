package report

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Generator struct {
	step *reportGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{step: newReportGenerator(deps)}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := g.step.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Report),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}
