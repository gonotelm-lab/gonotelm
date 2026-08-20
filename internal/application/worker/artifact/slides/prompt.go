package slides

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed slides-outline.jinja
var slidesOutlinePromptContent string

//go:embed slides.jinja
var slidesPromptContent string

var slidesOutlineTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(slidesOutlinePromptContent))

var slidesTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(slidesPromptContent))

type OutlineSource struct {
	Id       string
	Abstract string
}

func sourceIdsFrom(sources []OutlineSource) []string {
	ids := make([]string, 0, len(sources))
	for _, s := range sources {
		ids = append(ids, s.Id)
	}
	return ids
}

func RenderSlidesOutline(
	ctx context.Context,
	sources []OutlineSource,
	lang artifactentity.Language,
	tip string,
) ([]*einoschema.Message, error) {
	msgs, err := slidesOutlineTpl.Format(ctx, map[string]any{
		"SourceIds": types.NormalizeStrings(sourceIdsFrom(sources)),
		"Sources":   sources,
		"Language":  lang.DisplayName(),
	})
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
	sources []OutlineSource,
	runtime, workspaceDir string,
	outputLocation string,
	visualStyle artifactentity.SlidesVisualStyle,
	lang artifactentity.Language,
	tip string,
) ([]*einoschema.Message, error) {
	if !visualStyle.Supported() {
		visualStyle = artifactentity.SlidesVisualStyleDefaultValue()
	}
	msgs, err := slidesTpl.Format(ctx, map[string]any{
		"Title":          title,
		"Outline":        outline,
		"SourceIds":      types.NormalizeStrings(sourceIdsFrom(sources)),
		"Sources":        sources,
		"Runtime":        runtime,
		"WorkspaceDir":   workspaceDir,
		"OutputLocation": outputLocation,
		"VisualStyle":    visualStyle.String(),
		"Language":       lang.DisplayName(),
	})
	if err != nil {
		return nil, fmt.Errorf("render slides prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}

	return msgs, nil
}
