package infographic

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed studio-infographic.jinja
var infographicPromptContent string

var infographicTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(infographicPromptContent))

type TemplateVars struct {
	SourceIds    []string
	TextLanguage string
	ExtraPrompt  string
	Orientation  string
	DetailLevel  string
}

func normalizeOrientation(orientation string) string {
	normalized := strings.ToLower(strings.TrimSpace(orientation))
	switch normalized {
	case "portrait", "landscape", "square":
		return normalized
	default:
		return "landscape"
	}
}

func normalizeDetailLevel(level string) string {
	normalized := strings.ToLower(strings.TrimSpace(level))
	switch normalized {
	case "concise", "standard", "detailed":
		return normalized
	default:
		return "standard"
	}
}

func (v TemplateVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds":    types.NormalizeStrings(v.SourceIds),
		"TextLanguage": strings.TrimSpace(v.TextLanguage),
		"ExtraPrompt":  strings.TrimSpace(v.ExtraPrompt),
		"Orientation":  normalizeOrientation(v.Orientation),
		"DetailLevel":  normalizeDetailLevel(v.DetailLevel),
	}
}

func RenderInfographic(ctx context.Context, vars TemplateVars) ([]*einoschema.Message, error) {
	msgs, err := infographicTpl.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render infographic prompt: %w", err)
	}
	return msgs, nil
}
