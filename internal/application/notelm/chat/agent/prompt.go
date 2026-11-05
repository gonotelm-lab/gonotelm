package agent

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	chatvo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/vo"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed prompt.jinja
var promptTemplateContent string

var promptTemplate = prompt.FromMessages(
	einoschema.Jinja2,
	einoschema.SystemMessage(promptTemplateContent),
)

type PromptSource struct {
	Id       string
	Name     string
	Abstract string
}

type PromptTemplateVars struct {
	Notebook     string
	Style        chatvo.Style
	AnswerLength chatvo.AnswerLength
	Sources      []PromptSource
}

func (v PromptTemplateVars) promptVars() map[string]any {
	style := v.Style.String()
	if style == "" {
		style = chatvo.StyleDefault.String()
	}

	answerLength := v.AnswerLength.String()
	if answerLength == "" {
		answerLength = chatvo.AnswerLengthDefault.String()
	}

	return map[string]any{
		"Notebook":     strings.TrimSpace(v.Notebook),
		"Style":        style,
		"AnswerLength": answerLength,
		"Sources":      v.Sources,
	}
}

func renderSystemPrompt(ctx context.Context, vars PromptTemplateVars) (*einoschema.Message, error) {
	msgs, err := promptTemplate.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render chat agent prompt: %w", err)
	}
	if len(msgs) == 0 {
		return nil, fmt.Errorf("expected one prompt message, got %d", len(msgs))
	}

	return msgs[0], nil
}

func formatNotebookInfo(name, description string) string {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)

	switch {
	case name != "" && description != "":
		return fmt.Sprintf("Name：%s\nDescription：%s", name, description)
	case name != "":
		return fmt.Sprintf("Name：%s", name)
	case description != "":
		return fmt.Sprintf("Description：%s", description)
	default:
		return ""
	}
}
