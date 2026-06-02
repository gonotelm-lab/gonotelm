package prompts

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestStudioMindmapTemplateVarsPromptVars(t *testing.T) {
	promptVars := StudioMindmapTemplateVars{
		Mode:      "  ABSTRACT  ",
		Contents:  []string{" 文本A ", " ", "\n文本B\n"},
		Abstracts: []string{" 导图A ", " ", "\n导图B\n"},
	}.PromptVars()

	mode, ok := promptVars["Mode"].(string)
	if !ok {
		t.Fatalf("mode should be string")
	}
	if mode != StudioMindmapModeAbstract {
		t.Fatalf("unexpected mode: %s", mode)
	}

	contents, ok := promptVars["Contents"].([]string)
	if !ok {
		t.Fatalf("contents should be []string")
	}
	if len(contents) != 2 || contents[0] != "文本A" || contents[1] != "文本B" {
		t.Fatalf("unexpected contents: %v", contents)
	}

	abstracts, ok := promptVars["Abstracts"].([]string)
	if !ok {
		t.Fatalf("abstracts should be []string")
	}
	if len(abstracts) != 2 || abstracts[0] != "导图A" || abstracts[1] != "导图B" {
		t.Fatalf("unexpected abstracts: %v", abstracts)
	}
}

func TestStudioMindmapTemplateVarsPromptVars_DefaultMode(t *testing.T) {
	promptVars := StudioMindmapTemplateVars{
		Mode: "unknown",
	}.PromptVars()

	mode, ok := promptVars["Mode"].(string)
	if !ok {
		t.Fatalf("mode should be string")
	}
	if mode != StudioMindmapModeContent {
		t.Fatalf("unexpected default mode: %s", mode)
	}
}

func TestStudioMindmapMessageWithMode_Content(t *testing.T) {
	msg, err := StudioMindmapMessageWithMode(
		context.Background(),
		StudioMindmapModeContent,
		[]string{"Rust 所有权核心规则"},
		nil,
		"zh",
	)
	if err != nil {
		t.Fatalf("render studio mindmap(content) failed: %v", err)
	}

	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}

	if !strings.Contains(msg.Content, "Rust 所有权核心规则") {
		t.Fatalf("render result does not contain content source")
	}
}

func TestStudioMindmapMessageWithMode_Abstract(t *testing.T) {
	msg, err := StudioMindmapMessageWithMode(
		context.Background(),
		StudioMindmapModeAbstract,
		nil,
		[]string{"mindmap\n  root((所有权))\n    借用"},
		"zh",
	)
	if err != nil {
		t.Fatalf("render studio mindmap(abstract) failed: %v", err)
	}

	if msg.Role != schema.System {
		t.Fatalf("unexpected role: %s", msg.Role)
	}

	if !strings.Contains(msg.Content, "导图1") {
		t.Fatalf("render result does not contain abstract source section")
	}
	if !strings.Contains(msg.Content, "root((所有权))") {
		t.Fatalf("render result does not contain abstract payload")
	}
}

func TestFormatStudioMindmapResult(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name: "valid minimal",
			content: "```mermaid\n" +
				"mindmap\n" +
				"  root((信息不足))\n" +
				"```",
			want: true,
		},
		{
			name: "valid with branches",
			content: "```mermaid\n" +
				"mindmap\n" +
				"  root((Rust核心概念))\n" +
				"    所有权\n" +
				"      规则\n" +
				"    借用\n" +
				"```",
			want: true,
		},
		{
			name:    "invalid empty",
			content: "",
			want:    false,
		},
		{
			name: "invalid has extra text outside block",
			content: "这是解释文字\n" +
				"```mermaid\n" +
				"mindmap\n" +
				"  root((主题))\n" +
				"```",
			want: false,
		},
		{
			name: "invalid non mermaid block",
			content: "```markdown\n" +
				"mindmap\n" +
				"  root((主题))\n" +
				"```",
			want: false,
		},
		{
			name: "invalid first line not mindmap",
			content: "```mermaid\n" +
				"graph TD\n" +
				"  A-->B\n" +
				"```",
			want: false,
		},
		{
			name: "invalid no root node",
			content: "```mermaid\n" +
				"mindmap\n" +
				"  一级主题\n" +
				"```",
			want: false,
		},
		{
			name: "invalid multiple roots",
			content: "```mermaid\n" +
				"mindmap\n" +
				"  root((主题A))\n" +
				"  root((主题B))\n" +
				"```",
			want: false,
		},
		{
			name: "invalid root is not first node",
			content: "```mermaid\n" +
				"mindmap\n" +
				"  一级主题\n" +
				"  root((主题))\n" +
				"```",
			want: false,
		},
		{
			name: "invalid nested fence in body",
			content: "```mermaid\n" +
				"mindmap\n" +
				"  root((主题))\n" +
				"  ```\n" +
				"```",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckStudioMindmapResult(tt.content)
			if got != tt.want {
				t.Fatalf("got %v, want %v, content=%q", got, tt.want, tt.content)
			}
		})
	}
}
