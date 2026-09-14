package slides

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxservice "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/service"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type sandboxProvider struct {
	deps *types.WorkerDeps
}

func newSandboxProvider(deps *types.WorkerDeps) *sandboxProvider {
	return &sandboxProvider{deps: deps}
}

// 保留沙箱等待自然过期
func (p *sandboxProvider) ensure(ctx context.Context, req *types.Request) (sandboxent.Sandbox, error) {
	ss, err := p.getService()
	if err != nil {
		return nil, err
	}

	sandbox, err := ss.GetOrCreateSandbox(ctx,
		sandboxent.SandboxKey{
			UserId:     req.UserId,
			NotebookId: req.NotebookId,
		}, sandboxent.Spec{
			Env: map[string]string{
				"GONOTELM_WORKER_ARTIFACT_SCENE": "slides",
			},
		},
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "slides worker get or create sandbox failed, notebookId=%s", req.NotebookId)
	}

	return sandbox, nil
}

func (p *sandboxProvider) getService() (*sandboxservice.Service, error) {
	mgr, err := p.deps.Sandbox.GetManager(conf.WorkerGlobal().Studio.Slides.SandboxProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "slides worker get provider failed")
	}

	service := sandboxservice.New(p.deps.SandboxRepository, mgr, p.deps.DistLock)
	return service, nil
}

// ensureSlidesWorkspace 在沙箱 notebook/studioslides 下创建 artifact 子目录，并挂上 vendor 软链。
//
// sandbox 工作目录结构如下：
//
//	/tmp/{userId}/{notebookId}/                 ← 沙箱 WorkspaceDir（Bash 默认 cwd、真实 vendor）
//	├── vendor/                                 ← 沙箱上传的 pptxgenjs
//	└── studioslides/
//	    └── {artifactId}/                       ← slides 逻辑工作区（prompt WorkspaceDir）
//	        ├── vendor -> ../../vendor          ← 软链，兼容 slides/../vendor 的 require
//	        └── slides/
//	            ├── slide-01.js
//	            ├── compile.js
//	            └── output/presentation.pptx
func ensureSlidesWorkspace(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	vendorLink := path.Join(workspaceDir, "vendor")
	slidesOut := path.Join(workspaceDir, "slides", "output")
	// 建工作区 + vendor 软链；并校验 notebook 级 vendor/standalone.cjs 非空（0 字节会导致 agent 全盘找库）
	cmd := fmt.Sprintf(
		"mkdir -p %s && ln -sfn ../../vendor %s && test -s %s/standalone.cjs",
		shellQuote(slidesOut),
		shellQuote(vendorLink),
		shellQuote(vendorLink),
	)
	exec, err := sandbox.Run(ctx, sandboxent.Command{Command: cmd})
	if err != nil {
		return errors.WithMessagef(err, "mkdir slides workspace failed: %s", workspaceDir)
	}
	if !exec.Success() {
		return errors.Errorf(
			"slides workspace not ready (vendor/standalone.cjs missing or empty?): exit=%d stderr=%s",
			exec.ExitCode, string(exec.Stderr),
		)
	}
	return nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
