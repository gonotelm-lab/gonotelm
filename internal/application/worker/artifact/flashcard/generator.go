package flashcard

import (
	"context"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type Generator struct {
	step *flashcardGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{step: newFlashcardGenerator(deps)}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := g.step.generate(ctx, req)
	if err != nil {
		return nil, err
	}

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
