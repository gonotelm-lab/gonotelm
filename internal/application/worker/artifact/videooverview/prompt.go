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

var (
	//go:embed video-script.jinja
	videoScriptPromptContent string

	//go:embed video-storyboard.jinja
	videoStoryboardPromptContent string

	//go:embed video-generate.jinja
	videoGeneratePromptContent string

	//go:embed video-shot-subagent.jinja
	videoShotSubagentPromptContent string
)

var (
	videoScriptTpl       = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoScriptPromptContent))
	videoStoryboardTpl   = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoStoryboardPromptContent))
	videoGenerateTpl     = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoGeneratePromptContent))
	videoShotSubagentTpl = prompt.FromMessages(einoschema.Jinja2, einoschema.SystemMessage(videoShotSubagentPromptContent))
)

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
	subagentConcurrency int,
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
		"SubagentConcurrency":   subagentConcurrency,
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

func RenderVideoShotSubagentSystemPrompt(
	ctx context.Context,
	lang artifactentity.Language,
	style artifactentity.VideoOverviewStyle,
	workspaceDir string,
) (string, error) {
	commonKnowledge := renderShotCommonKnowledge(style)

	msgs, err := videoShotSubagentTpl.Format(ctx, map[string]any{
		"Language":        lang.DisplayName(),
		"CJKFont":         lang == artifactentity.LanguageChinese,
		"VisualStyle":     style.String(),
		"WorkspaceDir":    workspaceDir,
		"CommonKnowledge": commonKnowledge,
	})
	if err != nil {
		return "", fmt.Errorf("render video shot subagent prompt: %w", err)
	}
	if len(msgs) == 0 {
		return "", fmt.Errorf("render video shot subagent prompt: no messages")
	}
	return strings.TrimSpace(msgs[0].Content), nil
}

func shotThemeFiles(style artifactentity.VideoOverviewStyle) (themeJSON, tokensCSS string) {
	if !style.Supported() {
		style = artifactentity.VideoOverviewStyleDefault
	}

	prefix := "skills/theme/" + style.String()
	themeJSONRaw, _ := shotThemeFS.ReadFile(prefix + "/theme.json")
	tokensCSSRaw, _ := shotThemeFS.ReadFile(prefix + "/tokens.css")
	return string(themeJSONRaw), string(tokensCSSRaw)
}

// 通用知识放进系统提示，所有 subagent 共享同一段缓存前缀。
func renderShotCommonKnowledge(style artifactentity.VideoOverviewStyle) string {
	themeJSON, tokensCSS := shotThemeFiles(style)
	themePrefix := "skills/theme/" + style.String()

	sections := []struct{ path, body string }{
		{"skills/hyperframes-contract/SKILL.md", shotContractSkill},
		{"skills/visual-style/SKILL.md", shotVisualStyleSkill},
		{"skills/animation/SKILL.md", shotAnimationSkill},
		{themePrefix + "/theme.json", themeJSON},
		{themePrefix + "/tokens.css", tokensCSS},
	}

	var builder strings.Builder
	for _, section := range sections {
		fmt.Fprintf(&builder, "\n<common_knowledge path=%q>\n", section.path)
		builder.WriteString(section.body)
		if !strings.HasSuffix(section.body, "\n") {
			builder.WriteString("\n")
		}
		builder.WriteString("</common_knowledge>\n")
	}
	return strings.TrimSpace(builder.String())
}
