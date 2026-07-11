package report

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed title-maker.jinja
var titleMakerPromptContent string

var titleMakerTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(titleMakerPromptContent))

type TitleMakerVars struct {
	TitleLenRange string
	Text          string
}

func (v TitleMakerVars) promptVars() map[string]any {
	return map[string]any{
		"TitleLenRange": v.TitleLenRange,
		"Text":          strings.TrimSpace(v.Text),
	}
}

func RenderTitleMaker(ctx context.Context, text string) ([]*einoschema.Message, error) {
	vars := TitleMakerVars{
		TitleLenRange: "10-25",
		Text:          text,
	}
	msgs, err := titleMakerTpl.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render title maker prompt: %w", err)
	}
	return msgs, nil
}
