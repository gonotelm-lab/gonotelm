package videooverview

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed video-script.jinja
var videoScriptPromptContent string

//go:embed video-storyboard.jinja
var videoStoryboardPromptContent string

var videoScriptTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoScriptPromptContent))
var videoStoryboardTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoStoryboardPromptContent))

func RenderVideoScript(
	ctx context.Context,
	sourceIds []string,
	lang artifactentity.Language,
	tip string,
	style artifactentity.VideoOverviewStyle,
) ([]*einoschema.Message, error) {
	msgs, err := videoScriptTpl.Format(ctx, map[string]any{
		"SourceIds": types.NormalizeStrings(sourceIds),
		"Language":  lang.DisplayName(),
		"StyleInfo": map[string]any{
			"Style":       style.String(),
			"Description": videoStyleDescription(style),
		},
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
	style artifactentity.VideoOverviewStyle,
	outlineMarkdown string,
	narrationMarkdown string,
) ([]*einoschema.Message, error) {
	msgs, err := videoStoryboardTpl.Format(ctx, map[string]any{
		"SourceIds":         types.NormalizeStrings(sourceIds),
		"Language":          lang.DisplayName(),
		"OutlineMarkdown":   outlineMarkdown,
		"NarrationMarkdown": narrationMarkdown,
		"StyleInfo": map[string]any{
			"Style":       style.String(),
			"Description": videoStyleDescription(style),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("render video storyboard prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	return msgs, nil
}

func videoStyleDescription(style artifactentity.VideoOverviewStyle) string {
	switch style {
	case artifactentity.VideoOverviewStyleEducational:
		return "面向学习者，结构清晰，重点突出，讲解耐心，适合知识科普"
	case artifactentity.VideoOverviewStyleCute:
		return "轻松活泼，语言亲切，多用比喻和趣味表达，适合入门介绍"
	default:
		return "简洁专业，信息密度适中，逻辑顺畅，适合通用知识讲解"
	}
}
