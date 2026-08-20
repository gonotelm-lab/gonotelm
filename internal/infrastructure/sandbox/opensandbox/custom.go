package opensandbox

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"

	osb "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

type CustomSandbox struct {
	s       *osb.Sandbox
	key     entity.SandboxKey
	runtime string
}

var _ entity.Sandbox = &CustomSandbox{}

// opensandbox/code-interpreter-base
func NewCustomSandbox(s *osb.Sandbox, key entity.SandboxKey, runtime string) *CustomSandbox {
	return &CustomSandbox{s: s, key: key, runtime: runtime}
}

func (a *CustomSandbox) Id() string {
	return a.s.ID()
}

func (a *CustomSandbox) Description() entity.SandboxDescription {
	return entity.SandboxDescription{
		Id:      a.s.ID(),
		Key:     a.key,
		Runtime: a.runtime,
	}
}

func (a *CustomSandbox) Run(ctx context.Context, cmd entity.Command) (entity.Execution, error) {
	req := osb.RunCommandRequest{
		Command: cmd.Command,
		Cwd:     cmd.Cwd,
		Envs:    cmd.Env,
	}
	if req.Cwd == "" {
		req.Cwd = a.key.WorkspaceDir()
	}
	if cmd.Timeout > 0 {
		req.Timeout = int64(cmd.Timeout.Seconds())
	}

	start := time.Now()
	exec, err := a.s.RunCommandWithOpts(ctx, req, nil)
	duration := time.Since(start)
	if err != nil {
		return entity.Execution{}, pkgerr.Wrapf(err, "custom sandbox run command (%s) failed", cmd.Command)
	}

	var exitCode int
	if exec.ExitCode != nil {
		exitCode = *exec.ExitCode
	}

	return entity.Execution{
		ExitCode: exitCode,
		Stdout:   []byte(exec.Text()),
		Stderr:   []byte(formatOutputMessages(exec.Stderr)),
		Duration: duration,
	}, nil
}

// WriteFile 写入文件
func (a *CustomSandbox) WriteFile(ctx context.Context, path string, data io.Reader) error {
	err := a.s.UploadFile(ctx, data, osb.UploadFileOptions{
		Metadata: osb.FileMetadata{
			Path: path,
			Mode: 755,
		},
	})
	if err != nil {
		return pkgerr.Wrapf(err, "custom sandbox upload file (%s) failed", path)
	}

	return nil
}

// ReadFile 读取文件全部内容
func (a *CustomSandbox) ReadFile(ctx context.Context, path string, opts ...entity.SandboxOption) ([]byte, error) {
	rc, err := a.ReadFile2(ctx, path, opts...)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "custom sandbox io.ReadAll (%s) failed", path)
	}

	return body, nil
}

// ReadFile2 读取文件全部内容，返回流式读取器，调用方负责关闭
func (a *CustomSandbox) ReadFile2(ctx context.Context, path string, opts ...entity.SandboxOption) (io.ReadCloser, error) {
	option := entity.GetSandboxOption(opts...)
	rangeHeader := ""
	if option.Bytes > 0 {
		rangeHeader = fmt.Sprintf("bytes=0-%d", option.Bytes) // 0-N字节
	}
	rc, err := a.s.DownloadFile(ctx, path, rangeHeader) // http.header的Range
	if err != nil {
		return nil, pkgerr.Wrapf(err, "custom sandbox download file (%s) failed", path)
	}

	return rc, nil
}

// ListDir 列出目录内容
func (a *CustomSandbox) ListDir(ctx context.Context, dir string) ([]entity.ListDirItem, error) {
	fileInfos, err := a.s.ListDirectory(ctx, dir)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "custom sandbox list directory (%s) failed", dir)
	}

	items := make([]entity.ListDirItem, 0, len(fileInfos))
	for _, fi := range fileInfos {
		items = append(items, entity.ListDirItem{
			Path:       fi.Path,
			Type:       fi.Type,
			Size:       fi.Size,
			ModifiedAt: fi.ModifiedAt,
			CreatedAt:  fi.CreatedAt,
		})
	}

	return items, nil
}

// EditFile 编辑文件，将 Old 内容替换为 New 内容
func (a *CustomSandbox) EditFile(ctx context.Context, path, old, new string) error {
	if path == "" || old == "" {
		return pkgerr.Wrapf(pkgerr.ErrParams, "edit file: path and old content are required")
	}

	resp, err := a.s.ReplaceInFilesDetailed(ctx, osb.ReplaceRequest{
		path: {Old: old, New: new},
	})
	if err != nil {
		return pkgerr.Wrapf(err, "edit file %s replace failed", path)
	}
	if resp[path].ReplacedCount == 0 {
		return pkgerr.Wrapf(pkgerr.ErrParams, "edit file %s: old content not found", path)
	}

	return nil
}

func formatOutputMessages(msgs []osb.OutputMessage) string {
	var b strings.Builder
	for i, m := range msgs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.Text)
	}
	return b.String()
}
