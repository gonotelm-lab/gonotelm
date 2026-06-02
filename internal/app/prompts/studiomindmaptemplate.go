package prompts

import (
	"context"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/schema"
)

var studioMindmapRootLineRegexp = regexp.MustCompile(`^\s*root\(\(.+\)\)\s*$`)

const (
	StudioMindmapModeContent  = "content"
	StudioMindmapModeAbstract = "abstract"
)

type StudioMindmapTemplateVars struct {
	Mode      string
	Contents  []string
	Abstracts []string
}

func (v StudioMindmapTemplateVars) PromptVars() map[string]any {
	// 预处理一下abstracts
	vals := make(map[string]any)
	vals["Mode"] = v.Mode
	if v.Mode == StudioMindmapModeAbstract {
		abstracts := make([]string, 0, len(v.Abstracts))
		for _, str := range v.Abstracts {
			str = strings.TrimSpace(str)
			if str == "" {
				continue
			}

			str = strings.TrimPrefix(str, "```mermaid")
			str = strings.TrimSuffix(str, "```")
			if str == "" {
				continue
			}

			abstracts = append(abstracts, str)
		}
		vals["Abstracts"] = abstracts
	} else {
		vals["Contents"] = normalizeStrings(v.Contents)
	}

	return vals
}

type StudioMindmapTemplate = template[StudioMindmapTemplateVars]

func NewStudioMindmapTemplate(lang string) *StudioMindmapTemplate {
	return newTemplate[StudioMindmapTemplateVars](templateNameStudioMindmap, lang)
}

func StudioMindmapContentMessage(
	ctx context.Context,
	contents []string,
	lang string,
) (*schema.Message, error) {
	return StudioMindmapMessageWithMode(
		ctx,
		StudioMindmapModeContent,
		contents,
		nil,
		lang,
	)
}

func StudioMindmapAbstractMessage(
	ctx context.Context,
	abstracts []string,
	lang string,
) (*schema.Message, error) {
	return StudioMindmapMessageWithMode(
		ctx,
		StudioMindmapModeAbstract,
		nil,
		abstracts,
		lang,
	)
}

func StudioMindmapMessageWithMode(
	ctx context.Context,
	mode string,
	contents []string,
	abstracts []string,
	lang string,
) (*schema.Message, error) {
	tmpl := NewStudioMindmapTemplate(lang)
	msg, err := tmpl.Message(ctx,
		StudioMindmapTemplateVars{
			Mode:      mode,
			Contents:  contents,
			Abstracts: abstracts,
		})
	if err != nil {
		return nil, err
	}

	return msg, nil
}

// 检查大模型返回的思维导图输出是否符合格式
func CheckStudioMindmapResult(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	lines := strings.Split(normalized, "\n")
	if len(lines) < 3 {
		return false
	}

	// 只允许一个 mermaid 代码块，首行必须是 ```mermaid，末行必须是 ```
	if strings.TrimSpace(lines[0]) != "```mermaid" {
		return false
	}
	if strings.TrimSpace(lines[len(lines)-1]) != "```" {
		return false
	}

	bodyLines := lines[1 : len(lines)-1]
	nonEmptyBodyLines := make([]string, 0, len(bodyLines))
	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, "```") {
			// body 中再次出现 fence 说明不是单一代码块输出
			return false
		}
		nonEmptyBodyLines = append(nonEmptyBodyLines, line)
	}

	// 最少包含：mindmap + root((...))
	if len(nonEmptyBodyLines) < 2 {
		return false
	}

	// mindmap 语法要求：首行必须是 mindmap
	if strings.TrimSpace(nonEmptyBodyLines[0]) != "mindmap" {
		return false
	}

	nodeLines := nonEmptyBodyLines[1:]
	// root 必须是第一个节点，且全图仅能有一个 root
	if !studioMindmapRootLineRegexp.MatchString(nodeLines[0]) {
		return false
	}

	rootCount := 0
	for _, line := range nodeLines {
		if studioMindmapRootLineRegexp.MatchString(line) {
			rootCount++
		}
	}
	if rootCount != 1 {
		return false
	}

	return true
}
