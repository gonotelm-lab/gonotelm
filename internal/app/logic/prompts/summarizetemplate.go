package prompts

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

const defaultSummarizeInstruction = "你是一个优秀的文本摘要助手，使用简短的文字概括以下的内容"

type SummarizeTemplateVars struct {
	Text string
}

func (v SummarizeTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"text": strings.TrimSpace(v.Text),
	}
}

type SummarizeTemplate = template[SummarizeTemplateVars]

func NewSummarizeTemplate(lang string) (*SummarizeTemplate, error) {
	return newTemplate[SummarizeTemplateVars](templateNameSummarize, lang), nil
}

func SummarizePromptMessage(ctx context.Context, text, lang string) *schema.Message {
	tmpl, err := NewSummarizeTemplate(lang)
	if err != nil {
		return schema.SystemMessage(fallbackSummarizePrompt(text))
	}

	msg, err := tmpl.Message(ctx, SummarizeTemplateVars{Text: text})
	if err != nil {
		return schema.SystemMessage(fallbackSummarizePrompt(text))
	}

	return msg
}

func SummarizePrompt(text, lang string) string {
	return SummarizePromptMessage(context.Background(), text, lang).Content
}

func fallbackSummarizePrompt(text string) string {
	normalizedText := strings.TrimSpace(text)
	if normalizedText == "" {
		return defaultSummarizeInstruction
	}

	return defaultSummarizeInstruction + "\n\n" + normalizedText
}
