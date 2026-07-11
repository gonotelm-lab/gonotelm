package agent

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

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
	Style        ChatMessageStyle
	AnswerLength ChatMessageAnswerLength
	Sources      []PromptSource
}

func (v PromptTemplateVars) promptVars() map[string]any {
	style := string(v.Style)
	if style == "" {
		style = string(ChatMessageStyleDefault)
	}

	answerLength := string(v.AnswerLength)
	if answerLength == "" {
		answerLength = string(ChatMessageAnswerLengthDefault)
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
		return fmt.Sprintf("名称：%s\n描述：%s", name, description)
	case name != "":
		return fmt.Sprintf("名称：%s", name)
	case description != "":
		return fmt.Sprintf("描述：%s", description)
	default:
		return ""
	}
}
