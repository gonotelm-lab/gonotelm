package videooverview

import (
	"bytes"
	"context"
	"embed"
	"io/fs"
	"path"
	"strings"

	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

//go:embed skills
var videoSkillsFS embed.FS

// 单镜 subagent 的通用知识：固定技能逐文件嵌入（路径写错即编译错误），
// 主题随 visual_style 变，整个目录嵌入。
//
//go:embed skills/hyperframes-contract/SKILL.md
var shotContractSkill string

//go:embed skills/visual-style/SKILL.md
var shotVisualStyleSkill string

//go:embed skills/animation/SKILL.md
var shotAnimationSkill string

//go:embed skills/theme
var shotThemeFS embed.FS

// syncSkillsToSandbox 把内置 Agent Skills 按目录结构写入沙箱工作目录（幂等覆盖）。
func syncSkillsToSandbox(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	return syncEmbeddedTree(ctx, sandbox, workspaceDir, videoSkillsFS, "skills")
}

// syncEmbeddedTree 把 embed.FS 下 root 目录按结构写入沙箱工作目录（幂等覆盖）。
func syncEmbeddedTree(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string, fsys fs.FS, root string) error {
	var (
		dirs  []string
		files []string
	)

	err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path.Join(workspaceDir, p))
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return errors.WithMessagef(err, "walk embedded %s failed", root)
	}

	if len(dirs) > 0 {
		quoted := make([]string, 0, len(dirs))
		for _, dir := range dirs {
			quoted = append(quoted, shellQuote(dir))
		}
		exec, err := sandbox.Run(ctx, sandboxent.Command{Command: "mkdir -p " + strings.Join(quoted, " ")})
		if err != nil {
			return errors.WithMessagef(err, "create sandbox %s directories failed", root)
		}
		if !exec.Success() {
			return errors.Errorf(
				"create sandbox %s directories failed: exit=%d stderr=%s",
				root, exec.ExitCode, string(exec.Stderr),
			)
		}
	}

	for _, p := range files {
		content, err := fs.ReadFile(fsys, p)
		if err != nil {
			return errors.WithMessagef(err, "read embedded %s failed: %s", root, p)
		}
		dest := path.Join(workspaceDir, p)
		if err := sandbox.WriteFile(ctx, dest, bytes.NewReader(content)); err != nil {
			return errors.WithMessagef(err, "write %s to sandbox failed: %s", root, dest)
		}
	}

	return nil
}
