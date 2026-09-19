package videooverview

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"strings"
	"testing"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
)

type stubSkillSandbox struct {
	sandboxent.Sandbox
	writes map[string][]byte
	cmds   []string
}

func (s *stubSkillSandbox) Run(_ context.Context, cmd sandboxent.Command) (sandboxent.Execution, error) {
	s.cmds = append(s.cmds, cmd.Command)
	return sandboxent.Execution{ExitCode: 0}, nil
}

func (s *stubSkillSandbox) WriteFile(_ context.Context, filePath string, data io.Reader) error {
	content, err := io.ReadAll(data)
	if err != nil {
		return err
	}
	s.writes[filePath] = content
	return nil
}

func TestSyncSkillsToSandbox(t *testing.T) {
	sb := &stubSkillSandbox{writes: map[string][]byte{}}
	workspaceDir := "/tmp/user/notebook/studiovideooverview/artifact"

	if err := syncSkillsToSandbox(context.Background(), sb, workspaceDir); err != nil {
		t.Fatalf("syncSkillsToSandbox failed: %v", err)
	}

	var (
		wantFiles []string
		skillMD   int
		themeJSON int
		themeCSS  int
	)
	err := fs.WalkDir(videoSkillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		wantFiles = append(wantFiles, p)
		switch {
		case d.Name() == "SKILL.md":
			skillMD++
		case strings.HasSuffix(p, "/theme.json"):
			themeJSON++
		case strings.HasSuffix(p, "/tokens.css"):
			themeCSS++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded skills failed: %v", err)
	}
	if skillMD != 7 {
		t.Fatalf("unexpected embedded skill count: got %d, want 7", skillMD)
	}
	if themeJSON != 3 || themeCSS != 3 {
		t.Fatalf("unexpected embedded theme files: theme.json=%d tokens.css=%d, want 3 each", themeJSON, themeCSS)
	}

	for _, p := range wantFiles {
		key := workspaceDir + "/" + p
		content, ok := sb.writes[key]
		if !ok {
			t.Fatalf("skill not written to sandbox: %s", key)
		}
		if len(bytes.TrimSpace(content)) == 0 {
			t.Fatalf("skill written empty: %s", p)
		}
	}
	if len(sb.writes) != len(wantFiles) {
		t.Fatalf("write count mismatch: wrote %d, embedded %d", len(sb.writes), len(wantFiles))
	}
	if len(sb.cmds) != 1 || !strings.Contains(sb.cmds[0], "mkdir -p") {
		t.Fatalf("expected one mkdir command, got %v", sb.cmds)
	}
}

func TestRenderVideoGenerateIncludesSkillCatalog(t *testing.T) {
	msgs, err := RenderVideoGenerate(
		context.Background(),
		artifactentity.LanguageChinese,
		"",
		artifactentity.VideoOverviewStyleDefault,
		"linux",
		"/tmp/ws",
		"/tmp/ws/output/video.mp4",
		"storyboard",
		"audio",
	)
	if err != nil {
		t.Fatalf("RenderVideoGenerate failed: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatal("no messages rendered")
	}
	content := msgs[0].Content

	for _, name := range []string{"hyperframes-contract", "visual-style", "animation", "camera", "data-viz", "transitions", "components"} {
		if !strings.Contains(content, "<name>"+name+"</name>") {
			t.Errorf("system prompt missing skill metadata: %s", name)
		}
		if !strings.Contains(content, "skills/"+name+"/SKILL.md") {
			t.Errorf("system prompt missing skill location: %s", name)
		}
	}
	if strings.Contains(content, "<name>examples</name>") {
		t.Error("examples must not be listed as a skill")
	}
	if !strings.Contains(content, "ReadFile") {
		t.Error("system prompt missing ReadFile activation guidance")
	}
	if !strings.Contains(content, "# 接线速查") || !strings.Contains(content, "compositions/shot-01.html") {
		t.Error("system prompt missing composition wiring section")
	}
	if !strings.Contains(content, "hyperframes-contract/SKILL.md") {
		t.Error("system prompt must point at the contract skill for full skeletons")
	}
	if !strings.Contains(content, "skills/theme/default/theme.json") {
		t.Error("system prompt missing theme.json location")
	}
	if !strings.Contains(content, "skills/theme/default/tokens.css") {
		t.Error("system prompt missing tokens.css location")
	}
	if !strings.Contains(content, "var(--surface)") {
		t.Error("system prompt example must use theme tokens")
	}
	if strings.Contains(content, "PaletteTable") || strings.Contains(content, "StyleInfo") {
		t.Error("system prompt must not depend on injected palette")
	}
}

// TestGeneratePromptPrefersEditFile 锁定「改已写好的 HTML 用 EditFile、不要写 shell 补丁」这条纪律。
func TestGeneratePromptPrefersEditFile(t *testing.T) {
	msgs, err := RenderVideoGenerate(
		context.Background(),
		artifactentity.LanguageChinese,
		"",
		artifactentity.VideoOverviewStyleDefault,
		"linux",
		"/tmp/ws",
		"/tmp/ws/output/video.mp4",
		"storyboard",
		"audio",
	)
	if err != nil {
		t.Fatalf("RenderVideoGenerate failed: %v", err)
	}
	content := msgs[0].Content
	for _, want := range []string{
		"改**已经写好**的 HTML 用 `EditFile`",
		"不要整份 `WriteFile` 重写",
		"并行发多个 `EditFile`",
		"逐镜对照着写",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("generate prompt missing authoring discipline: %s", want)
		}
	}
}

// TestHyperframesContractCoversSilentAntiPatterns 锁定 lint 抓不到、但必须第一稿避开的写法清单。
func TestHyperframesContractCoversSilentAntiPatterns(t *testing.T) {
	raw, err := fs.ReadFile(videoSkillsFS, "skills/hyperframes-contract/SKILL.md")
	if err != nil {
		t.Fatalf("read contract skill failed: %v", err)
	}
	content := string(raw)
	for _, want := range []string{
		"lint 抓不到",
		"fromTo` 只用来定义「入场初始态」",
		"`tl.set(el, { … }, t)`",
		"`tl.to(el, { x: 76, … }, t)`",
		"strokeDasharray",
		"data-duration - 0.1 ~ 0.3s",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("contract skill missing anti-pattern rule: %s", want)
		}
	}
}
