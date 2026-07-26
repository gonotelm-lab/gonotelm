package quiz

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed quiz.jinja
var quizPromptContent string

var quizTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(quizPromptContent))

type RenderVars struct {
	SourceIds  []string
	Count      string
	Difficulty string
	Tip        string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds":  types.NormalizeStrings(v.SourceIds),
		"Count":      v.Count,
		"Difficulty": v.Difficulty,
		"Tip":        v.Tip,
	}
}

func RenderQuiz(
	ctx context.Context,
	sourceIds []string,
	count artifactentity.QuizCount,
	difficulty artifactentity.QuizDifficulty,
	tip string,
) ([]*einoschema.Message, error) {
	if !count.Supported() {
		count = artifactentity.QuizCountDefaultValue()
	}
	if !difficulty.Supported() {
		difficulty = artifactentity.QuizDifficultyDefault()
	}
	msgs, err := quizTpl.Format(ctx, RenderVars{
		SourceIds:  sourceIds,
		Count:      count.String(),
		Difficulty: difficulty.String(),
		Tip:        tip,
	}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render quiz prompt: %w", err)
	}
	return msgs, nil
}
