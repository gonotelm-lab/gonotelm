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

	var wantFiles []string
	err := fs.WalkDir(videoSkillsFS, "skills", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			wantFiles = append(wantFiles, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded skills failed: %v", err)
	}
	if len(wantFiles) != 7 {
		t.Fatalf("unexpected embedded skill count: got %d, want 7", len(wantFiles))
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
	if !strings.Contains(content, "# 完整示例") || !strings.Contains(content, "compositions/shot-01.html") {
		t.Error("system prompt missing inline example section")
	}
}
