package videooverview

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed video-script.jinja
var videoScriptPromptContent string

//go:embed video-storyboard.jinja
var videoStoryboardPromptContent string

//go:embed video-generate.jinja
var videoGeneratePromptContent string

var videoScriptTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoScriptPromptContent))
var videoStoryboardTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoStoryboardPromptContent))
var videoGenerateTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoGeneratePromptContent))

func RenderVideoScript(
	ctx context.Context,
	sourceIds []string,
	lang artifactentity.Language,
	tip string,
) ([]*einoschema.Message, error) {
	msgs, err := videoScriptTpl.Format(ctx, map[string]any{
		"SourceIds": types.NormalizeStrings(sourceIds),
		"Language":  lang.DisplayName(),
		"Tip":       strings.TrimSpace(tip),
	})
	if err != nil {
		return nil, fmt.Errorf("render video script prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	return msgs, nil
}

func RenderVideoStoryboard(
	ctx context.Context,
	sourceIds []string,
	lang artifactentity.Language,
	tip string,
	segmentsMarkdown string,
) ([]*einoschema.Message, error) {
	msgs, err := videoStoryboardTpl.Format(ctx, map[string]any{
		"SourceIds":        types.NormalizeStrings(sourceIds),
		"Language":         lang.DisplayName(),
		"SegmentsMarkdown": segmentsMarkdown,
		"Tip":              strings.TrimSpace(tip),
	})
	if err != nil {
		return nil, fmt.Errorf("render video storyboard prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	return msgs, nil
}

func RenderVideoGenerate(
	ctx context.Context,
	lang artifactentity.Language,
	tip string,
	style artifactentity.VideoOverviewStyle,
	runtime string,
	workspaceDir string,
	outputLocation string,
	storyboardMarkdown string,
	audioManifestMarkdown string,
) ([]*einoschema.Message, error) {
	msgs, err := videoGenerateTpl.Format(ctx, map[string]any{
		"Language":              lang.DisplayName(),
		"CJKFont":               lang == artifactentity.LanguageChinese,
		"VisualStyle":           style.String(),
		"Runtime":               runtime,
		"WorkspaceDir":          workspaceDir,
		"OutputLocation":        outputLocation,
		"StoryboardMarkdown":    storyboardMarkdown,
		"AudioManifestMarkdown": audioManifestMarkdown,
		"Skills":                pkgagent.SkillsPrompt(ctx, videoSkillsFS, "skills"),
		"Tip":                   strings.TrimSpace(tip),
	})
	if err != nil {
		return nil, fmt.Errorf("render video generate prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	return msgs, nil
}
