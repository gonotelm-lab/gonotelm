package quiz

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
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

type quizGenerator struct {
	deps *types.WorkerDeps
}

func newQuizGenerator(deps *types.WorkerDeps) *quizGenerator {
	return &quizGenerator{deps: deps}
}

func quizCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `quiz`",
		"`quiz` must include `questions`, `themes`, and `follow_up_hint`",
		"each question must have exactly 4 non-empty `options`",
		"`answer_index` must be non-empty; values must be unique integers in 0-3",
		"each question must include a non-empty `explanation` (why correct / why distractors are wrong)",
		"single-choice first (`answer_index` length 1), then multi-choice (length >= 2)",
		"`title` length preferably 10-30 characters",
	}
	if validateErr != nil {
		rules = append(rules, "Previous validation error: "+validateErr.Error())
	}
	return rules
}

func (g *quizGenerator) generate(
	ctx context.Context,
	req *types.Request,
) (*quizExpectation, error) {
	p := artifactentity.PayloadAs[*artifactentity.QuizPayload](req.Payload)
	count := artifactentity.QuizCountDefaultValue()
	if p.Count.Supported() {
		count = p.Count
	}
	difficulty := artifactentity.QuizDifficultyDefault()
	if p.Difficulty.Supported() {
		difficulty = p.Difficulty
	}

	msgs, err := RenderQuiz(ctx, types.SourceIDsToStrings(req.SourceIds), count, difficulty, p.GetTip())
	if err != nil {
		return nil, errors.WithMessagef(err, "generate quiz message failed")
	}

	ag, err := newQuizAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*quizExpectation](ag, "quiz").
		WithParse(g.parse).
		WithRetry(quizMaxCompensateRetry).
		WithRules(quizCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func (g *quizGenerator) parse(ctx context.Context, content string) (*quizExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect quizExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "quiz direct unmarshal did not match, fallback to json extraction",
				slog.String("err", types.TruncateForLog(err.Error())),
				slog.String("raw_content", types.TruncateForLog(content)))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "quiz output unmarshal failed after compatibility fallback",
			slog.String("err", types.TruncateForLog(err.Error())),
			slog.String("raw_content", types.TruncateForLog(content)))
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
