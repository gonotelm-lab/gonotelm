package prompt

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	einoschema "github.com/cloudwego/eino/schema"
)

//go:embed studio-mindmap-v2.jinja
var mindmapPromptContent string

//go:embed studio-report.jinja
var reportPromptContent string

//go:embed studio-infographic.jinja
var infographicPromptContent string

//go:embed studio-podcast-outline.jinja
var podcastOutlinePromptContent string

//go:embed title-maker.jinja
var titleMakerPromptContent string

var mindmapTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(mindmapPromptContent))
var reportTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(reportPromptContent))
var infographicTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(infographicPromptContent))
var podcastOutlineTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(podcastOutlinePromptContent))
var titleMakerTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(titleMakerPromptContent))

type RenderMindmapVars struct {
	SourceIds []string
}

func (v RenderMindmapVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": normalizeStrings(v.SourceIds),
	}
}

func RenderMindmap(ctx context.Context, sourceIds []string) ([]*einoschema.Message, error) {
	msgs, err := mindmapTpl.Format(ctx, RenderMindmapVars{SourceIds: sourceIds}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render mindmap prompt: %w", err)
	}
	return msgs, nil
}

type RenderReportVars struct {
	SourceIds []string
}

func (v RenderReportVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds": normalizeStrings(v.SourceIds),
	}
}

func RenderReport(ctx context.Context, sourceIds []string) ([]*einoschema.Message, error) {
	msgs, err := reportTpl.Format(ctx, RenderReportVars{SourceIds: sourceIds}.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render report prompt: %w", err)
	}
	return msgs, nil
}

type StudioInfoGraphicTemplateVars struct {
	SourceIds    []string
	TextLanguage string
	ExtraPrompt  string
	Orientation  string
	DetailLevel  string
}

func (v StudioInfoGraphicTemplateVars) promptVars() map[string]any {
	return map[string]any{
		"SourceIds":    normalizeStrings(v.SourceIds),
		"TextLanguage": strings.TrimSpace(v.TextLanguage),
		"ExtraPrompt":  strings.TrimSpace(v.ExtraPrompt),
		"Orientation":  normalizeOrientation(v.Orientation),
		"DetailLevel":  normalizeDetailLevel(v.DetailLevel),
	}
}

func RenderInfographic(ctx context.Context, vars StudioInfoGraphicTemplateVars) ([]*einoschema.Message, error) {
	msgs, err := infographicTpl.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render infographic prompt: %w", err)
	}
	return msgs, nil
}

func RenderPodcastOutline(ctx context.Context, sourceIds []string, lang string, tips string, style PodcastStyle) ([]*einoschema.Message, error) {
	vars := StudioPodcastOutlineTemplateVars{
		SourceIds: sourceIds,
		Language:  lang,
		Tips:      tips,
	}
	info, ok := builtinPodcastInfos[style]
	if !ok {
		info = builtinPodcastInfos[PodcastStyleAbstract]
	}
	vars.Style = info.Style
	vars.StyleDesc = info.Description
	vars.Speakers = info.Speakers
	vars.NumOfSegments = info.NumOfSegments

	msgs, err := podcastOutlineTpl.Format(ctx, vars.PromptVars())
	if err != nil {
		return nil, fmt.Errorf("render podcast outline prompt: %w", err)
	}
	return msgs, nil
}

type RenderTitleMakerVars struct {
	TitleLenRange string
	Text          string
}

func (v RenderTitleMakerVars) promptVars() map[string]any {
	return map[string]any{
		"TitleLenRange": v.TitleLenRange,
		"Text":          strings.TrimSpace(v.Text),
	}
}

func RenderTitleMaker(ctx context.Context, text string) ([]*einoschema.Message, error) {
	vars := RenderTitleMakerVars{
		TitleLenRange: "10-25",
		Text:          text,
	}
	msgs, err := titleMakerTpl.Format(ctx, vars.promptVars())
	if err != nil {
		return nil, fmt.Errorf("render title maker prompt: %w", err)
	}
	return msgs, nil
}
