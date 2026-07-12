package infographic

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

//go:embed studio-infographic.jinja
var infographicPromptContent string

var infographicTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(infographicPromptContent))

type TemplateVars struct {
	SourceIds    []string
	TextLanguage string
	ExtraPrompt  string
	Orientation  artifactentity.ArtifactInfoGraphicOrientation
	DetailLevel  artifactentity.ArtifactInfoGraphicDetailLevel
}

func (v TemplateVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds":    types.NormalizeStrings(v.SourceIds),
		"TextLanguage": strings.TrimSpace(v.TextLanguage),
		"ExtraPrompt":  strings.TrimSpace(v.ExtraPrompt),
		"Orientation":  v.Orientation.String(),
		"DetailLevel":  v.DetailLevel.String(),
	}
}

func RenderInfographic(ctx context.Context, vars TemplateVars) ([]*einoschema.Message, error) {
	msgs, err := infographicTpl.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render infographic prompt: %w", err)
	}
	return msgs, nil
}
