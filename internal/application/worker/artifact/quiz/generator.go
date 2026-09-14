package quiz

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/bytedance/sonic"
)

type Generator struct {
	step *quizGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{step: newQuizGenerator(deps)}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := g.step.generate(ctx, req)
	if err != nil {
		return nil, err
	}

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
