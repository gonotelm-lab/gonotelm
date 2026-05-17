package prompts

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type SummarizeTemplateVars struct {
	Text string
}

func (v SummarizeTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"text": strings.TrimSpace(v.Text),
	}
}

type SummarizeTemplate = template[SummarizeTemplateVars]

func NewSummarizeTemplate(lang string) (*SummarizeTemplate) {
	return newTemplate[SummarizeTemplateVars](templateNameSummarize, lang)
}

func SummarizePromptMessage(ctx context.Context, text, lang string) (*schema.Message, error) {
	tmpl := NewSummarizeTemplate(lang)
	msg, err := tmpl.Message(ctx, SummarizeTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}

	return msg, nil
}

func SummarizePrompt(text, lang string) (string, error) {
	msg, err := SummarizePromptMessage(context.Background(), text, lang)
	if err != nil {
		return "", err
	}

	return msg.Content, nil
}
