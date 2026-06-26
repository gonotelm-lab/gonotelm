package prompt

import (
	"strings"
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
