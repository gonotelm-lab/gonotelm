package prompt

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type TitleMakerTemplateVars struct {
	Text   string
	MaxLen int
	MinLen int
}

func (v TitleMakerTemplateVars) PromptVars() map[string]any {
	if v.MaxLen <= 0 {
		v.MaxLen = 25
	}
	if v.MinLen <= 0 {
		v.MinLen = 10
	}
	if v.MinLen > v.MaxLen {
		v.MinLen, v.MaxLen = v.MaxLen, v.MinLen
	}

	titleLenRange := fmt.Sprintf("%d-%d", v.MinLen, v.MaxLen)
	if v.MinLen == v.MaxLen {
		titleLenRange = fmt.Sprintf("%d", v.MinLen)
	}

	return map[string]any{
		"TitleLenRange": titleLenRange,
		"Text":          strings.TrimSpace(v.Text),
	}
}

type TitleMakerTemplate = template[TitleMakerTemplateVars]

func NewTitleMakerTemplate(lang string) *TitleMakerTemplate {
	return newTemplate[TitleMakerTemplateVars](templateNameTitleMaker, lang)
}

func RenderTitleMakerMessage(ctx context.Context, text, lang string) ([]*schema.Message, error) {
	tmpl := NewTitleMakerTemplate(lang)
	msg, err := tmpl.Message(ctx, TitleMakerTemplateVars{Text: text})
	if err != nil {
		return nil, err
	}

	return prependSystemMessage([]*schema.Message{msg}), nil
}
