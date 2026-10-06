package types

import (
	"bytes"
	"context"
	"io/fs"
	"path"
	"strings"

	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// SyncEmbeddedTree 把 fsys 下 root 目录按结构写入沙箱工作目录（幂等覆盖）。
// 各产物用它把内置知识库（skills/、components/ 等）投递进沙箱工作区。
func SyncEmbeddedTree(
	ctx context.Context,
	sandbox sandboxent.Sandbox,
	workspaceDir string,
	fsys fs.FS,
	root string,
) error {
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
			quoted = append(quoted, sandboxShellQuote(dir))
		}
		exec, err := sandbox.Run(ctx, sandboxent.Command{
			Command: "mkdir -p " + strings.Join(quoted, " "),
		})
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

func sandboxShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
