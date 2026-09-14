package datatable

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Generator struct {
	step *dataTableGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{step: newDataTableGenerator(deps)}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := g.step.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Table),
		ResultKind: entity.ResultKindInline,
	}, nil
}
