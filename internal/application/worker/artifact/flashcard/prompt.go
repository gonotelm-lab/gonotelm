package flashcard

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed flashcard.jinja
var flashcardPromptContent string

var flashcardTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(flashcardPromptContent))

type RenderVars struct {
	SourceIds  []string
	Count      string
	Difficulty string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds":  types.NormalizeStrings(v.SourceIds),
		"Count":      v.Count,
		"Difficulty": v.Difficulty,
	}
}

func RenderFlashcard(
	ctx context.Context,
	sourceIds []string,
	count artifactentity.FlashcardCount,
	difficulty artifactentity.FlashcardDifficulty,
	tip string,
) ([]*einoschema.Message, error) {
	if !count.Supported() {
		count = artifactentity.FlashcardCountDefaultValue()
	}
	if !difficulty.Supported() {
		difficulty = artifactentity.FlashcardDifficultyDefault()
	}
	msgs, err := flashcardTpl.Format(ctx, RenderVars{
		SourceIds:  sourceIds,
		Count:      count.String(),
		Difficulty: difficulty.String(),
	}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render flashcard prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	
	return msgs, nil
}
