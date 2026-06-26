package prompt

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

type StudioInfoGraphicTemplateVars struct {
	SourceIds    []string
	TextLanguage string
	ExtraPrompt  string
	Orientation  string
	DetailLevel  string
}

func (v StudioInfoGraphicTemplateVars) PromptVars() map[string]any {
	return map[string]any{
		"SourceIds":    normalizeStrings(v.SourceIds),
		"TextLanguage": strings.TrimSpace(v.TextLanguage),
		"ExtraPrompt":  strings.TrimSpace(v.ExtraPrompt),
		"Orientation":  normalizeStudioInfoGraphicOrientation(v.Orientation),
		"DetailLevel":  normalizeStudioInfoGraphicDetailLevel(v.DetailLevel),
	}
}

func normalizeStudioInfoGraphicOrientation(orientation string) string {
	normalized := strings.ToLower(strings.TrimSpace(orientation))
	switch normalized {
	case "portrait", "landscape", "square":
		return normalized
	default:
		return "landscape"
	}
}

func normalizeStudioInfoGraphicDetailLevel(level string) string {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "concise", "standard", "detailed":
		return normalized
	default:
		return "standard"
	}
}

type StudioInfoGraphicTemplate = template[StudioInfoGraphicTemplateVars]

func NewStudioInfoGraphicTemplate(lang string) *StudioInfoGraphicTemplate {
	return newTemplate[StudioInfoGraphicTemplateVars](templateNameStudioInfographic, lang)
}

func RenderStudioInfoGraphicMessage(
	ctx context.Context,
	vars StudioInfoGraphicTemplateVars,
	lang string,
) ([]*schema.Message, error) {
	tmpl := NewStudioInfoGraphicTemplate(lang)
	msg, err := tmpl.Message(ctx, vars)
	if err != nil {
		return nil, err
	}

	return prependSystemMessage([]*schema.Message{msg}), nil
}
