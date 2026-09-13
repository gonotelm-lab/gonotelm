package agent

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const skillFileName = "SKILL.md"

const skillsPromptHeader = "The following skills provide specialized instructions for specific tasks.\n" +
	"When a task matches a skill's description, load the skill's SKILL.md for detailed instructions."

type skillMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Location    string `yaml:"-"`
}

// SkillsPrompt 扫描 fsys 中 root 下的标准 Agent Skills（<skill>/SKILL.md），
// 返回可放入 system prompt 的元数据字符串；无有效 skill 时返回 ""。
func SkillsPrompt(ctx context.Context, fsys fs.FS, root string) string {
	skills := collectSkills(ctx, fsys, root)
	if len(skills) == 0 {
		return ""
	}

	slices.SortFunc(skills, func(a, b skillMeta) int { return cmp.Compare(a.Name, b.Name) })

	var b strings.Builder
	b.WriteString(skillsPromptHeader)
	b.WriteString("\n\n<available_skills>\n")
	for _, s := range skills {
		fmt.Fprintf(&b, "  <skill>\n    <name>%s</name>\n    <description>%s</description>\n    <location>%s</location>\n  </skill>\n",
			html.EscapeString(s.Name),
			html.EscapeString(s.Description),
			html.EscapeString(s.Location),
		)
	}
	b.WriteString("</available_skills>")

	return b.String()
}

func collectSkills(ctx context.Context, fsys fs.FS, root string) []skillMeta {
	var (
		skills []skillMeta
		seen   = make(map[string]struct{})
	)

	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != skillFileName {
			return nil
		}

		meta, ok := parseSkillFile(fsys, p)
		if !ok {
			slog.WarnContext(ctx, "skip invalid skill", slog.String("path", p))
			return nil
		}
		if _, dup := seen[meta.Name]; dup {
			slog.WarnContext(ctx, "skip duplicated skill",
				slog.String("name", meta.Name), slog.String("path", p))
			return nil
		}

		meta.Location = p
		seen[meta.Name] = struct{}{}
		skills = append(skills, meta)
		return nil
	})
	if err != nil {
		slog.WarnContext(ctx, "walk skills failed", slog.String("root", root), slog.Any("err", err))
		return nil
	}

	return skills
}

func parseSkillFile(fsys fs.FS, p string) (skillMeta, bool) {
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		return skillMeta{}, false
	}
	return parseSkill(data)
}

// extractFrontmatter 提取文件开头被 --- 行包裹的 YAML 块。
func extractFrontmatter(data []byte) ([]byte, bool) {
	lines := bytes.Split(data, []byte("\n"))
	if len(lines) == 0 || strings.TrimSpace(string(lines[0])) != "---" {
		return nil, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(string(lines[i])) == "---" {
			return bytes.Join(lines[1:i], []byte("\n")), true
		}
	}
	return nil, false
}

// parseSkill 从 SKILL.md 内容解析 name / description，任一缺失即视为无效。
func parseSkill(data []byte) (skillMeta, bool) {
	fm, ok := extractFrontmatter(data)
	if !ok {
		return skillMeta{}, false
	}

	var meta skillMeta
	if err := yaml.Unmarshal(fm, &meta); err != nil {
		return skillMeta{}, false
	}

	meta.Name = strings.TrimSpace(meta.Name)
	meta.Description = strings.TrimSpace(meta.Description)
	if meta.Name == "" || meta.Description == "" {
		return skillMeta{}, false
	}

	return meta, true
}
