package flashcard

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
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
	deps *types.ServiceDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
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
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (g *Generator) llmOptions() []einomodel.Option {
	var (
		provider = conf.WorkerGlobal().Studio.Flashcard.ModelProvider
		model    = conf.WorkerGlobal().Studio.Flashcard.Model
	)
	return []einomodel.Option{
		chat.WithModel(model),
		chat.WithResponseJsonObject(provider),
		chat.WithThinking(provider, false),
	}
}

func (g *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (*flashcardExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioFlashcardScene)
	llmOptions := g.llmOptions()

	count := artifactentity.FlashcardCountDefaultValue()
	difficulty := artifactentity.FlashcardDifficultyDefault()
	tip := ""
	if p, ok := req.Payload.(*artifactentity.FlashcardPayload); ok {
		if p.Count.Supported() {
			count = p.Count
		}
		if p.Difficulty.Supported() {
			difficulty = p.Difficulty
		}
		tip = p.Tip
	}

	ag, err := types.BuildSourceExploreAgent(
		g.deps,
		conf.WorkerGlobal().Studio.Flashcard.ModelProvider,
		conf.WorkerGlobal().Studio.Flashcard.Model,
		conf.WorkerGlobal().Studio.Flashcard.MaxRound,
		llmOptions,
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "failed to build source explore agent for flashcard, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderFlashcard(ctx, sourceIds, count, difficulty, tip)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate flashcard message failed, err=%v", err)
	}
	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate flashcard output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate flashcard agent usage: %+v", ag.TokenUsage()))

	expect, err := parseAgentOutput(ctx, output.Content)
	if err == nil {
		return expect, nil
	}

	slog.WarnContext(ctx, "flashcard agent output invalid, compensating",
		slog.String("notebook_id", req.NotebookId.String()),
		slog.Any("usage", ag.TokenUsage()),
	)

	msgs = append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	msgs = append(msgs, types.BuildCompensateMessage(output.Content, []string{
		"JSON 字段必须且仅能包含 title 和 flashcard",
		"flashcard 必须仅包含 cards 数组",
		"每张卡必须包含 front、back、hint；front 与 back 不能为空",
		"title 长度建议为 10-30 字",
	}))

	llmResp, genErr := ag.BaseLLM().Generate(ctx, msgs, llmOptions...)
	if genErr != nil {
		return nil, errors.Wrapf(errors.ErrLLM,
			"flashcard compensate generate failed, err=%v",
			genErr,
		)
	}

	expect, err = parseAgentOutput(ctx, llmResp.Content)
	if err == nil {
		return expect, nil
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"flashcard agent output invalid after compensation, first_output=%q, compensate_output=%q, err=%v",
		output.Content,
		llmResp.Content,
		err,
	)
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
				slog.Any("err", err))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "flashcard output unmarshal failed after compatibility fallback",
			slog.Any("err", err))
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
