package opensandbox

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"

	osb "github.com/alibaba/OpenSandbox/sdks/sandbox/go"
)

type alpineSandbox struct {
	s *osb.Sandbox
}

var _ entity.Sandbox = &alpineSandbox{}

func newAlpineSandbox(s *osb.Sandbox) *alpineSandbox {
	return &alpineSandbox{s: s}
}

func (a *alpineSandbox) Id() string {
	return a.s.ID()
}

func (a *alpineSandbox) Run(ctx context.Context, cmd entity.Command) (entity.Execution, error) {
	req := osb.RunCommandRequest{
		Command: cmd.Command,
		Cwd:     cmd.Cwd,
		Envs:    cmd.Env,
	}
	if cmd.Timeout > 0 {
		req.Timeout = int64(cmd.Timeout.Seconds())
	}

	start := time.Now()
	exec, err := a.s.RunCommandWithOpts(ctx, req, nil)
	duration := time.Since(start)
	if err != nil {
		return entity.Execution{}, pkgerr.Wrapf(err, "alpine sandbox run command (%s) failed", cmd.Command)
	}

	var exitCode int
	if exec.ExitCode != nil {
		exitCode = *exec.ExitCode
	}

	return entity.Execution{
		ExitCode: exitCode,
		Stdout:   []byte(exec.Text()),
		Stderr:   []byte(stderrText(exec.Stderr)),
		Duration: duration,
	}, nil
}

func stderrText(msgs []osb.OutputMessage) string {
	var b strings.Builder
	for i, m := range msgs {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(m.Text)
	}
	return b.String()
}

// WriteFile 写入文件
func (a *alpineSandbox) WriteFile(ctx context.Context, path string, data io.Reader) error {
	err := a.s.UploadFile(ctx, data, osb.UploadFileOptions{
		Metadata: osb.FileMetadata{
			Path: path,
			Mode: 0644,
		},
	})
	if err != nil {
		return pkgerr.Wrapf(err, "alpine sandbox upload file (%s) failed", path)
	}

	return nil
}

// ReadFile 读取文件全部内容
func (a *alpineSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	resp, err := a.s.DownloadFile(ctx, path, "") // 指定下载文件全部内容 这个返回的是http.Response.Body
	if err != nil {
		return nil, pkgerr.Wrapf(err, "alpine sandbox download file (%s) failed", path)
	}
	defer resp.Close()

	body, err := io.ReadAll(resp)
	if err != nil {
		return nil, pkgerr.Wrapf(err, "alpine sandbox io.ReadAll (%s) failed", path)
	}

	return body, nil
}
