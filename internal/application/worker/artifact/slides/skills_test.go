package slides

import (
	"context"
	"encoding/json"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
)

// 技能目录里每个 SKILL.md 都必须能被解析出 name/description 并出现在索引里：
// frontmatter 写坏时 loader 会静默跳过（只打 warn），模型就再也读不到这份知识。
func TestSlidesSkillsPrompt(t *testing.T) {
	got := pkgagent.SkillsPrompt(context.Background(), slidesSkillsFS, "skills")
	if got == "" {
		t.Fatal("skills index is empty: all SKILL.md files were skipped")
	}

	wantNames := []string{"pptxgenjs", "slides-layout", "slides-design", "slides-qa"}
	for _, name := range wantNames {
		if !strings.Contains(got, "<name>"+name+"</name>") {
			t.Errorf("skill %q missing from index:\n%s", name, got)
		}
		if !strings.Contains(got, "<location>skills/"+name+"/SKILL.md</location>") {
			t.Errorf("skill %q location missing from index:\n%s", name, got)
		}
	}
}

// 技能文件体积上限：单份过大时模型会读不完/读半截，需要拆成更小的技能。
func TestSlidesSkillFilesAreReadable(t *testing.T) {
	const maxSkillBytes = 24 * 1024

	err := fs.WalkDir(slidesSkillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Size() > maxSkillBytes {
			t.Errorf("%s is %d bytes (> %d): split it into smaller skills", p, info.Size(), maxSkillBytes)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk skills failed: %v", err)
	}
}

type slidesThemeFile struct {
	ID      string            `json:"id"`
	Units   map[string]string `json:"units"`
	Palette map[string]string `json:"palette"`
	Fonts   map[string]any    `json:"fonts"`
	Type    map[string]any    `json:"type"`
	Space   map[string]any    `json:"space"`
	Radius  map[string]any    `json:"radius"`
	Chart   struct {
		Colors []string `json:"colors"`
		Grid   string   `json:"grid"`
		Axis   string   `json:"axis"`
	} `json:"chart"`
	Guidance map[string]string `json:"guidance"`
}

// theme.json 是配色的唯一来源：它必须存在、可解析、色值合法，
// 且五个 palette 键齐全（页面代码与 compile.js 都按这个契约取值）。
func TestSlidesThemeFiles(t *testing.T) {
	hexRe := regexp.MustCompile(`^[0-9a-f]{6}$`)
	wantPalettes := map[string]map[string]string{
		"default": {
			"primary": "3d405b", "secondary": "6d6a5f", "accent": "e07a5f",
			"light": "f2cc8f", "bg": "faf6ec",
		},
		"educational": {
			"primary": "1e293b", "secondary": "475569", "accent": "0d9488",
			"light": "e2e8f0", "bg": "f8fafc",
		},
		"cute": {
			"primary": "5b4b6e", "secondary": "8b7aa0", "accent": "f4a4b8",
			"light": "b8e0d2", "bg": "fff8f5",
		},
	}

	for style, want := range wantPalettes {
		raw, err := fs.ReadFile(slidesSkillsFS, "skills/theme/"+style+"/theme.json")
		if err != nil {
			t.Fatalf("read theme %q failed: %v", style, err)
		}

		var theme slidesThemeFile
		if err := json.Unmarshal(raw, &theme); err != nil {
			t.Fatalf("theme %q is not valid json: %v", style, err)
		}

		if theme.ID != style {
			t.Errorf("theme %q has id %q", style, theme.ID)
		}
		for _, key := range []string{"primary", "secondary", "accent", "light", "bg"} {
			got, ok := theme.Palette[key]
			if !ok {
				t.Errorf("theme %q palette missing %q", style, key)
				continue
			}
			if !hexRe.MatchString(got) {
				t.Errorf("theme %q palette.%s = %q: must be 6-digit hex without '#'", style, key, got)
			}
			if want[key] != got {
				t.Errorf("theme %q palette.%s = %q, want %q (配色是产品契约，改动需同步设计文档)", style, key, got, want[key])
			}
		}
		if len(theme.Palette) != 5 {
			t.Errorf("theme %q palette has %d keys, want exactly 5", style, len(theme.Palette))
		}
		for _, group := range []string{"type", "space", "radius", "fonts", "guidance"} {
			var present bool
			switch group {
			case "type":
				present = len(theme.Type) > 0
			case "space":
				present = len(theme.Space) > 0
			case "radius":
				present = len(theme.Radius) > 0
			case "fonts":
				present = len(theme.Fonts) > 0
			case "guidance":
				present = len(theme.Guidance) > 0
			}
			if !present {
				t.Errorf("theme %q missing %q block", style, group)
			}
		}
		if len(theme.Chart.Colors) == 0 {
			t.Errorf("theme %q chart.colors is empty", style)
		}
		for _, c := range theme.Chart.Colors {
			if !hexRe.MatchString(c) {
				t.Errorf("theme %q chart color %q must be 6-digit hex without '#'", style, c)
			}
		}
	}
}

// 入口提示必须把技能索引注入进去、把主题文件指向唯一来源，
// 且不再复述主题内容（色值只能存在于 theme.json），渲染后不留模板标记。
func TestRenderSlidesPrompt(t *testing.T) {
	msgs, err := RenderSlides(
		context.Background(),
		"测试标题", "## 第一章\n### 页面一\n- 要点",
		[]OutlineSource{{Id: "src-1", Abstract: "摘要"}},
		"linux", "/tmp/ws/slides/artifact-1",
		"/tmp/ws/slides/artifact-1/slides/output/presentation.pptx",
		artifactentity.SlidesVisualStyleCute,
		artifactentity.LanguageChinese,
		"",
	)
	if err != nil {
		t.Fatalf("RenderSlides failed: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("RenderSlides returned no messages")
	}

	content := msgs[0].Content
	for _, want := range []string{
		"<available_skills>",
		"<name>slides-layout</name>",
		"<name>slides-qa</name>",
		"skills/slides-design/SKILL.md",
		"skills/theme/cute/theme.json", // 主题唯一来源（路径）
		`require(path.join(__dirname, "..", "skills", "theme", "cute", "theme.json"))`,
		"/tmp/ws/slides/artifact-1/slides/output/presentation.pptx",
		"分文件生成（强制）",       // 一页一文件
		"只写一页",            // 每次 WriteFile 只写一页
		"每轮并行写 **2–4 页**", // 分轮推进，避免一次生成过大
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendered prompt missing %q", want)
		}
	}

	// 主题内容只存在于 theme.json：提示里不应出现任何 palette 色值。
	for _, hex := range []string{"5b4b6e", "8b7aa0", "f4a4b8", "b8e0d2", "fff8f5", "3d405b", "1e293b"} {
		if strings.Contains(content, hex) {
			t.Errorf("rendered prompt leaks theme color %q: theme must come from theme.json only", hex)
		}
	}
	if strings.Contains(content, "{{") || strings.Contains(content, "{%") {
		t.Errorf("rendered prompt still contains template markers")
	}
	if strings.Contains(content, "deck-") {
		t.Errorf("rendered prompt references the old deck-* skill names")
	}
}
