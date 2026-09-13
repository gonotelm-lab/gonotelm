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
	style artifactentity.VideoOverviewStyle,
) ([]*einoschema.Message, error) {
	msgs, err := videoScriptTpl.Format(ctx, map[string]any{
		"SourceIds":   types.NormalizeStrings(sourceIds),
		"Language":    lang.DisplayName(),
		"VisualStyle": style.String(),
		"StyleInfo":   videoStyleInfo(style),
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
		"VisualStyle":       style.String(),
		"StyleInfo":         videoStyleInfo(style),
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
		"VisualStyle":           style.String(),
		"StyleInfo":             videoStyleInfo(style),
		"Runtime":               runtime,
		"WorkspaceDir":          workspaceDir,
		"OutputLocation":        outputLocation,
		"StoryboardMarkdown":    storyboardMarkdown,
		"AudioManifestMarkdown": audioManifestMarkdown,
	})
	if err != nil {
		return nil, fmt.Errorf("render video generate prompt: %w", err)
	}
	if tipMsg := types.BuildTipMessage(tip); tipMsg != nil {
		msgs = append(msgs, tipMsg)
	}
	return msgs, nil
}

func videoStyleInfo(style artifactentity.VideoOverviewStyle) map[string]any {
	return map[string]any{
		"Style":       style.String(),
		"Description": videoStyleDescription(style),
		"Palette":     videoStylePalette(style),
	}
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

// videoStylePalette 与 slides 同源（primary/secondary/accent/light/bg），全片只许本盘。
func videoStylePalette(style artifactentity.VideoOverviewStyle) map[string]string {
	switch style {
	case artifactentity.VideoOverviewStyleEducational:
		return map[string]string{
			"Primary":   "#1e293b",
			"Secondary": "#475569",
			"Accent":    "#0d9488",
			"Light":     "#e2e8f0",
			"Bg":        "#f8fafc",
			"Mood":      "教科书科普：高对比、几何分区清晰、冷静易读",
		}
	case artifactentity.VideoOverviewStyleCute:
		return map[string]string{
			"Primary":   "#5b4b6e",
			"Secondary": "#8b7aa0",
			"Accent":    "#f4a4b8",
			"Light":     "#b8e0d2",
			"Bg":        "#fff8f5",
			"Mood":      "粉嫩柔和（粉/薄荷绿/奶油底），治愈轻松",
		}
	default:
		return map[string]string{
			"Primary":   "#3d405b",
			"Secondary": "#6d6a5f",
			"Accent":    "#e07a5f",
			"Light":     "#f2cc8f",
			"Bg":        "#faf6ec",
			"Mood":      "扁平手绘感、米色/奶油纸张底、温暖教育向",
		}
	}
}
