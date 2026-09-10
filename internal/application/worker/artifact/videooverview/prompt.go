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

//go:embed video-outline.jinja
var videoOutlinePromptContent string

var videoOutlineTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoOutlinePromptContent))

//go:embed video-storyboard.jinja
var videoStoryboardPromptContent string

var videoStoryboardTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoStoryboardPromptContent))

func RenderVideoOutline(
	ctx context.Context,
	sourceIds []string,
	lang artifactentity.Language,
	tip string,
) ([]*einoschema.Message, error) {
	msgs, err := videoOutlineTpl.Format(ctx, map[string]any{
		"SourceIds": types.NormalizeStrings(sourceIds),
		"Language":  lang.DisplayName(),
	})
	if err != nil {
		return nil, fmt.Errorf("render video outline prompt: %w", err)
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
	outline *videoOutline,
) ([]*einoschema.Message, error) {
	segments := make([]map[string]string, 0, len(outline.Segments))
	for _, seg := range outline.Segments {
		segments = append(segments, map[string]string{
			"name":    seg.Name,
			"content": seg.Content,
		})
	}

	msgs, err := videoStoryboardTpl.Format(ctx, map[string]any{
		"SourceIds": types.NormalizeStrings(sourceIds),
		"Language":  lang.DisplayName(),
		"StyleInfo": map[string]any{
			"Style":       style.String(),
			"Description": videoStyleDescription(style),
		},
		"Outline": map[string]any{
			"title":    outline.Title,
			"segments": segments,
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
