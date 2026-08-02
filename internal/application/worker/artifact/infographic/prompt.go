package infographic

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

//go:embed infographic.jinja
var infoGraphicPromptContent string

var infoGraphicTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(infoGraphicPromptContent))

type TemplateVars struct {
	SourceIds    []string
	TextLanguage string
	Orientation  artifactentity.InfoGraphicOrientation
	DetailLevel  artifactentity.InfoGraphicDetailLevel
	VisualStyle  artifactentity.InfoGraphicVisualStyle
	ExtraPrompt  string
}

func (v TemplateVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds":    types.NormalizeStrings(v.SourceIds),
		"TextLanguage": strings.TrimSpace(v.TextLanguage),
		"Orientation":  v.Orientation.String(),
		"DetailLevel":  v.DetailLevel.String(),
		"VisualStyle":  v.VisualStyle.String(),
	}
}

func RenderInfographic(ctx context.Context, vars TemplateVars) ([]*einoschema.Message, error) {
	msgs, err := infoGraphicTpl.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render infographic prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(vars.ExtraPrompt); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	
	return msgs, nil
}
