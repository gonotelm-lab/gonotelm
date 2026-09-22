package agent

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"
)

func TestExtractFrontmatter(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{
			name:  "valid",
			input: "---\nname: a\n---\nbody",
			want:  "name: a",
			ok:    true,
		},
		{
			name:  "no frontmatter",
			input: "# body",
			ok:    false,
		},
		{
			name:  "unclosed",
			input: "---\nname: a\nbody",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := extractFrontmatter([]byte(tt.input))
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if string(got) != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseSkill(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  skillMeta
		ok    bool
	}{
		{
			name:  "valid",
			input: "---\nname: alpha\ndescription: Alpha skill\n---\n# Alpha\n",
			want:  skillMeta{Name: "alpha", Description: "Alpha skill"},
			ok:    true,
		},
		{
			name:  "trims whitespace",
			input: "---\nname: \"  alpha  \"\ndescription: \"  desc  \"\n---\n",
			want:  skillMeta{Name: "alpha", Description: "desc"},
			ok:    true,
		},
		{
			name:  "crlf",
			input: "---\r\nname: alpha\r\ndescription: Alpha skill\r\n---\r\n# Alpha\r\n",
			want:  skillMeta{Name: "alpha", Description: "Alpha skill"},
			ok:    true,
		},
		{
			name:  "missing description",
			input: "---\nname: alpha\n---\n",
			ok:    false,
		},
		{
			name:  "missing name",
			input: "---\ndescription: Alpha skill\n---\n",
			ok:    false,
		},
		{
			name:  "invalid yaml",
			input: "---\nname: [unclosed\n---\n",
			ok:    false,
		},
		{
			name:  "no frontmatter",
			input: "# Alpha\n",
			ok:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseSkill([]byte(tt.input))
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSkillsPrompt(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/alpha/SKILL.md": {Data: []byte("---\nname: alpha\ndescription: Alpha skill\n---\n# Alpha\n")},
		"skills/beta/SKILL.md":  {Data: []byte("---\nname: beta\ndescription: Beta skill\n---\n# Beta\n")},
		"skills/alpha/notes.md": {Data: []byte("# not a skill\n")},
	}

	got := SkillsPrompt(context.Background(), fsys, "skills")

	for _, want := range []string{
		"<name>alpha</name>",
		"<description>Alpha skill</description>",
		"<location>skills/alpha/SKILL.md</location>",
		"<name>beta</name>",
		"<description>Beta skill</description>",
		"<location>skills/beta/SKILL.md</location>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("prompt missing %q:\n%s", want, got)
		}
	}

	if strings.Index(got, "<name>alpha</name>") > strings.Index(got, "<name>beta</name>") {
		t.Fatalf("skills not sorted by name:\n%s", got)
	}
}

func TestSkillsPromptEscapesXML(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/escape/SKILL.md": {Data: []byte("---\nname: escape\ndescription: \"a & b < c\"\n---\n")},
	}

	got := SkillsPrompt(context.Background(), fsys, "skills")

	if !strings.Contains(got, "<description>a &amp; b &lt; c</description>") {
		t.Fatalf("description not escaped:\n%s", got)
	}
}

func TestSkillsPromptSkipsInvalid(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/good/SKILL.md":    {Data: []byte("---\nname: good\ndescription: Good skill\n---\n")},
		"skills/nodesc/SKILL.md":  {Data: []byte("---\nname: nodesc\n---\n")},
		"skills/nofm/SKILL.md":    {Data: []byte("# no frontmatter\n")},
		"skills/badyaml/SKILL.md": {Data: []byte("---\nname: [unclosed\n---\n")},
	}

	got := SkillsPrompt(context.Background(), fsys, "skills")

	if !strings.Contains(got, "<name>good</name>") {
		t.Fatalf("valid skill missing:\n%s", got)
	}
	for _, bad := range []string{"nodesc", "nofm", "badyaml"} {
		if strings.Contains(got, bad) {
			t.Fatalf("invalid skill %q leaked into prompt:\n%s", bad, got)
		}
	}
}

func TestSkillsPromptDedupes(t *testing.T) {
	fsys := fstest.MapFS{
		"skills/a/SKILL.md": {Data: []byte("---\nname: dup\ndescription: First\n---\n")},
		"skills/b/SKILL.md": {Data: []byte("---\nname: dup\ndescription: Second\n---\n")},
	}

	got := SkillsPrompt(context.Background(), fsys, "skills")

	if strings.Count(got, "<name>dup</name>") != 1 {
		t.Fatalf("expected one dup skill:\n%s", got)
	}
	if !strings.Contains(got, "First") || strings.Contains(got, "Second") {
		t.Fatalf("expected first walked skill kept:\n%s", got)
	}
}

func TestSkillsPromptEmpty(t *testing.T) {
	t.Run("missing root", func(t *testing.T) {
		got := SkillsPrompt(context.Background(), fstest.MapFS{}, "skills")
		if got != "" {
			t.Fatalf("expected empty prompt, got:\n%s", got)
		}
	})

	t.Run("no skills", func(t *testing.T) {
		fsys := fstest.MapFS{"skills/readme.md": {Data: []byte("hi")}}
		got := SkillsPrompt(context.Background(), fsys, "skills")
		if got != "" {
			t.Fatalf("expected empty prompt, got:\n%s", got)
		}
	})
}
