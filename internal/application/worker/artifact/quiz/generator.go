package quiz

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
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

const quizMaxCompensateRetry = 3

const quizOptionCount = 4

type QuizQuestion struct {
	Question    string   `json:"question"`
	Options     []string `json:"options"`
	AnswerIndex []int    `json:"answer_index"`
	Explanation string   `json:"explanation"`
}

type QuizContent struct {
	Questions    []QuizQuestion `json:"questions"`
	Themes       []string       `json:"themes"`
	FollowUpHint []string       `json:"follow_up_hint"`
}

type quizExpectation struct {
	Title string      `json:"title"`
	Quiz  QuizContent `json:"quiz"`
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

func (g *Generator) llmOptions() []einomodel.Option {
	var (
		provider = conf.WorkerGlobal().Studio.Quiz.ModelProvider
		model    = conf.WorkerGlobal().Studio.Quiz.Model
	)
	return []einomodel.Option{
		chat.WithModel(model),
		chat.WithResponseJsonObject(provider),
		chat.WithThinking(provider, false),
	}
}

func quizCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON 字段必须且仅能包含 title 和 quiz",
		"quiz 必须包含 questions、themes、follow_up_hint",
		"每题 options 必须恰好 4 个，且每个选项非空",
		"answer_index 不得为空，元素必须为 0-3 的整数且不重复、不越界",
		"每题必须包含非空 explanation（答案解析，说明为何正确/为何干扰项错误）",
		"必须先单选（answer_index 长度 1）再多选（长度 >= 2）",
		"title 长度建议为 10-30 字",
	}
	if validateErr != nil {
		rules = append(rules, "上次校验失败原因："+validateErr.Error())
	}
	return rules
}

func (g *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (*quizExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioQuizScene)
	llmOptions := g.llmOptions()

	count := artifactentity.QuizCountDefaultValue()
	difficulty := artifactentity.QuizDifficultyDefault()
	tip := ""
	if p, ok := req.Payload.(*artifactentity.QuizPayload); ok {
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
		conf.WorkerGlobal().Studio.Quiz.ModelProvider,
		conf.WorkerGlobal().Studio.Quiz.Model,
		conf.WorkerGlobal().Studio.Quiz.MaxRound,
		llmOptions,
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "failed to build source explore agent for quiz, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderQuiz(ctx, sourceIds, count, difficulty, tip)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate quiz message failed, err=%v", err)
	}
	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate quiz output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate quiz agent usage: %+v", ag.TokenUsage()))

	expect, parseErr := parseAgentOutput(ctx, output.Content)
	if parseErr == nil {
		return expect, nil
	}

	lastContent := output.Content
	lastErr := parseErr
	msgs = append([]*einoschema.Message{}, ag.AccumulatedMessages()...)

	for attempt := 1; attempt <= quizMaxCompensateRetry; attempt++ {
		slog.WarnContext(ctx, "quiz agent output invalid, compensating",
			slog.String("notebook_id", req.NotebookId.String()),
			slog.Int("attempt", attempt),
			slog.Int("max_retry", quizMaxCompensateRetry),
			slog.Any("err", lastErr),
			slog.Any("usage", ag.TokenUsage()),
		)

		compensateMsgs := append([]*einoschema.Message{}, msgs...)
		compensateMsgs = append(compensateMsgs, types.BuildCompensateMessage(lastContent, quizCompensateRules(lastErr)))

		llmResp, genErr := ag.BaseLLM().Generate(ctx, compensateMsgs, llmOptions...)
		if genErr != nil {
			return nil, errors.Wrapf(errors.ErrLLM,
				"quiz compensate generate failed on attempt %d, err=%v",
				attempt,
				genErr,
			)
		}

		expect, parseErr = parseAgentOutput(ctx, llmResp.Content)
		if parseErr == nil {
			return expect, nil
		}

		lastContent = llmResp.Content
		lastErr = parseErr
		// Continue conversation: previous compensate request + model reply.
		msgs = append(compensateMsgs, &einoschema.Message{
			Role:    einoschema.Assistant,
			Content: lastContent,
		})
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"quiz agent output invalid after %d retries, last_output=%q, err=%v",
		quizMaxCompensateRetry,
		lastContent,
		lastErr,
	)
}

func parseAgentOutput(ctx context.Context, content string) (*quizExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect quizExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "quiz direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", content))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "quiz output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", content))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	for i := range expect.Quiz.Questions {
		expect.Quiz.Questions[i].Question = strings.TrimSpace(expect.Quiz.Questions[i].Question)
		for j := range expect.Quiz.Questions[i].Options {
			expect.Quiz.Questions[i].Options[j] = strings.TrimSpace(expect.Quiz.Questions[i].Options[j])
		}
	}
	for i := range expect.Quiz.Themes {
		expect.Quiz.Themes[i] = strings.TrimSpace(expect.Quiz.Themes[i])
	}
	for i := range expect.Quiz.FollowUpHint {
		expect.Quiz.FollowUpHint[i] = strings.TrimSpace(expect.Quiz.FollowUpHint[i])
	}

	if expect.Title == "" {
		return nil, fmt.Errorf("title empty")
	}
	if err := ValidateQuizContent(expect.Quiz); err != nil {
		return nil, err
	}

	return &expect, nil
}
