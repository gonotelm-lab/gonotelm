package mindmap

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const MindmapMaxOnceToken = 32_000

type Generator struct {
	step *mindmapGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{step: newMindmapGenerator(deps)}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := g.step.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Mindmap),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}
