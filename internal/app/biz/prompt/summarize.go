package prompt

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type SummarizeTemplateVars struct {
	Text    string
	MaxWord int
	MinWord int
}

func (v SummarizeTemplateVars) PromptVars() map[string]any {
	if v.MaxWord <= 0 {
		v.MaxWord = 150
	}
	if v.MinWord <= 0 {
		v.MinWord = 60
	}

	wordRange := fmt.Sprintf("%d-%d", v.MinWord, v.MaxWord)

	if v.MaxWord == v.MinWord {
		wordRange = fmt.Sprintf("%d", v.MinWord)
	}
	return map[string]any{
		"WordRange": wordRange,
		"Text":      strings.TrimSpace(v.Text),
	}
}

type SummarizeTemplate = template[SummarizeTemplateVars]

func NewSummarizeTemplate(lang string) *SummarizeTemplate {
	return newTemplate[SummarizeTemplateVars](templateNameSummarize, lang)
}

func RenderSummarizeMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := NewSummarizeTemplate(lang)
	msg, err := tmpl.Message(ctx, SummarizeTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}

	return prependSystemMessage([]*schema.Message{msg}), nil
}
