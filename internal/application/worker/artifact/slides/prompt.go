package slides

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed slides-outline.jinja
var slidesOutlinePromptContent string

//go:embed slides.jinja
var slidesPromptContent string

var slidesOutlineTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(slidesOutlinePromptContent))

var slidesTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(slidesPromptContent))

type RenderVars struct {
	SourceIds []string
}

func (v RenderVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": types.NormalizeStrings(v.SourceIds),
	}
}

func RenderSlidesOutline(ctx context.Context, sourceIds []string, tip string) ([]*einoschema.Message, error) {
	msgs, err := slidesOutlineTpl.Format(ctx, RenderVars{SourceIds: sourceIds}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render slides outline prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}

	return msgs, nil
}

func RenderSlides(
	ctx context.Context,
	title, outline string,
	sourceIds []string,
	runtime, workspaceDir string,
	outputLocation string,
	tip string,
) ([]*einoschema.Message, error) {
	msgs, err := slidesTpl.Format(ctx, map[string]any{
		"Title":          title,
		"Outline":        outline,
		"SourceIds":      types.NormalizeStrings(sourceIds),
		"Runtime":        runtime,
		"WorkspaceDir":   workspaceDir,
		"OutputLocation": outputLocation,
	})
	if err != nil {
		return nil, fmt.Errorf("render slides prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}

	return msgs, nil
}
