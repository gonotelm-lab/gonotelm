package flashcard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type FlashcardCard struct {
	Front string `json:"front"`
	Back  string `json:"back"`
	Hint  string `json:"hint"`
}

type FlashcardContent struct {
	Cards []FlashcardCard `json:"cards"`
}

type flashcardExpectation struct {
	Title     string           `json:"title"`
	Flashcard FlashcardContent `json:"flashcard"`
}

type Generator struct {
	deps *types.WorkerDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{deps: deps}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := g.generate(ctx, req)
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

func (g *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (*flashcardExpectation, error) {
	p := entity.PayloadAs[*entity.FlashcardPayload](req.Payload)
	count := entity.FlashcardCountDefaultValue()
	if p.Count.Supported() {
		count = p.Count
	}
	difficulty := entity.FlashcardDifficultyDefault()
	if p.Difficulty.Supported() {
		difficulty = p.Difficulty
	}

	msgs, err := RenderFlashcard(ctx, types.SourceIDsToStrings(req.SourceIds), count, difficulty, p.GetTip())
	if err != nil {
		return nil, errors.WithMessagef(err, "generate flashcard message failed")
	}

	ag, err := newFlashcardAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*flashcardExpectation](ag, "flashcard").
		WithParse(parseAgentOutput).
		WithRules(flashcardCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func flashcardCompensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `flashcard`",
		"`flashcard` must contain only a `cards` array",
		"each card must include `front`, `back`, and `hint`; `front` and `back` must be non-empty",
		"`title` length preferably 10-30 characters",
	}
}

func parseAgentOutput(ctx context.Context, content string) (*flashcardExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect flashcardExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "flashcard direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "flashcard output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	for i := range expect.Flashcard.Cards {
		expect.Flashcard.Cards[i].Front = strings.TrimSpace(expect.Flashcard.Cards[i].Front)
		expect.Flashcard.Cards[i].Back = strings.TrimSpace(expect.Flashcard.Cards[i].Back)
		expect.Flashcard.Cards[i].Hint = strings.TrimSpace(expect.Flashcard.Cards[i].Hint)
	}

	if expect.Title == "" {
		return nil, fmt.Errorf("title empty")
	}
	if !CheckFlashcardContent(expect.Flashcard) {
		return nil, fmt.Errorf("flashcard content invalid")
	}

	return &expect, nil
}
