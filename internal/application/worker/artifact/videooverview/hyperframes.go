package videooverview

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
	einotoolutils "github.com/cloudwego/eino/components/tool/utils"
	"golang.org/x/sync/errgroup"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxservice "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type videoStorageResult struct {
	StoreKey    string `json:"store_key"`
	ContentType string `json:"content_type"`
}

type hyperframesVideoGenerator struct {
	deps *types.WorkerDeps
}

const (
	videoSandboxDefaultCPU    = "2"
	videoSandboxDefaultMemory = "4Gi"
)

const videoSandboxTTL = 3 * time.Hour

func videoSandboxResourceLimits() map[string]string {
	cfg := conf.WorkerGlobal().Studio.VideoOverview
	limits := map[string]string{
		"cpu":    videoSandboxDefaultCPU,
		"memory": videoSandboxDefaultMemory,
	}
	if cfg.SandboxCPU != "" {
		limits["cpu"] = cfg.SandboxCPU
	}
	if cfg.SandboxMemory != "" {
		limits["memory"] = cfg.SandboxMemory
	}
	return limits
}

func newHyperframesVideoGenerator(deps *types.WorkerDeps) *hyperframesVideoGenerator {
	return &hyperframesVideoGenerator{deps: deps}
}

func (g *hyperframesVideoGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	storyboardMarkdown string,
	audioMeta *audioCheckpointMeta,
) (*videoStorageResult, error) {
	sandbox, err := g.ensureSandbox(ctx, req)
	if err != nil {
		return nil, errors.WithMessage(err, "ensure video sandbox failed")
	}

	sandboxDesc := sandbox.Description()
	workspaceDir := path.Join(sandboxDesc.Key.WorkspaceDir(), types.StudioVideoOverviewsDir, req.ArtifactId.String())
	if err := ensureVideoOverviewWorkspace(ctx, sandbox, workspaceDir); err != nil {
		return nil, err
	}

	if err := syncSkillsToSandbox(ctx, sandbox, workspaceDir); err != nil {
		return nil, errors.WithMessage(err, "sync skills to sandbox failed")
	}

	if err := syncComponentsToSandbox(ctx, sandbox, workspaceDir); err != nil {
		return nil, errors.WithMessage(err, "sync components to sandbox failed")
	}

	if err := g.syncAudioToSandbox(ctx, sandbox, workspaceDir, audioMeta); err != nil {
		return nil, errors.WithMessage(err, "sync audio to sandbox failed")
	}

	agent, err := newHyperframesAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	checkMP4Tool, err := g.getCheckMP4ValidTool(sandbox)
	if err != nil {
		return nil, errors.WithMessage(err, "infer check mp4 tool failed")
	}

	if err := agent.AppendTools(map[string]einotool.InvokableTool{
		tools.BashToolName:      tools.NewBashTool(sandbox, workspaceDir),
		tools.ReadFileToolName:  tools.NewReadFileTool(sandbox),
		tools.WriteFileToolName: tools.NewWriteFileTool(sandbox),
		tools.EditFileToolName:  tools.NewEditFileTool(sandbox),
		tools.ListDirToolName:   tools.NewListDirTool(sandbox),
		checkMP4ValidToolName:   checkMP4Tool,
	}); err != nil {
		return nil, errors.Wrap(err, "hyperframes agent append tools failed")
	}

	outputLocation := path.Join(workspaceDir, "output", "video.mp4")
	msgs, err := RenderVideoGenerate(
		ctx,
		payload.GetLanguage(),
		payload.GetTip(),
		payload.GetVisualStyle(),
		sandboxDesc.Runtime,
		workspaceDir,
		outputLocation,
		storyboardMarkdown,
		renderAudioManifestMarkdown(workspaceDir, audioMeta),
	)
	if err != nil {
		return nil, errors.Wrap(err, "render video generate prompt failed")
	}

	if _, err := agent.React(ctx, msgs); err != nil {
		return nil, errors.Wrap(err, "generate hyperframes video failed")
	}
	slog.InfoContext(ctx, fmt.Sprintf("hyperframes video agent usage: %+v", agent.TokenUsage()))

	if err := checkMP4ArtifactValid(ctx, sandbox, outputLocation); err != nil {
		return nil, errors.WithMessagef(err, "hyperframes output invalid: %s", outputLocation)
	}

	mp4Reader, err := sandbox.ReadFile2(ctx, outputLocation)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := mp4Reader.Close(); cerr != nil {
			slog.WarnContext(ctx, "hyperframes mp4 reader close failed", slog.Any("err", cerr))
		}
	}()

	storeKey := formatVideoStoreKey(req.NotebookId, req.ArtifactId)
	if err := types.UploadReader(ctx, g.deps.ObjectStorage, storeKey, mimeTypeMP4, mp4Reader); err != nil {
		return nil, errors.Wrapf(err, "upload video object failed, artifact_id=%s", req.ArtifactId)
	}

	return &videoStorageResult{
		StoreKey:    storeKey,
		ContentType: mimeTypeMP4,
	}, nil
}

func (g *hyperframesVideoGenerator) ensureSandbox(ctx context.Context, req *types.Request) (sandboxent.Sandbox, error) {
	ss, err := g.getSandboxService()
	if err != nil {
		return nil, err
	}

	sandbox, err := ss.GetOrCreateSandbox(ctx,
		sandboxent.SandboxKey{
			UserId:     req.UserId,
			NotebookId: req.NotebookId,
		}, sandboxent.Spec{
			TTL: videoSandboxTTL,
			Env: map[string]string{
				"GONOTELM_WORKER_ARTIFACT_SCENE": "video_overview",
				// disable update check and telemetry for hyperframes CLI
				"HYPERFRAMES_NO_UPDATE_CHECK": "1",
				"HYPERFRAMES_NO_TELEMETRY":    "1",
			},
			ResourceLimits: videoSandboxResourceLimits(),
		},
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "video overview get or create sandbox failed, notebookId=%s", req.NotebookId)
	}
	return sandbox, nil
}

func (g *hyperframesVideoGenerator) getSandboxService() (*sandboxservice.Service, error) {
	mgr, err := g.deps.Sandbox.GetManager(conf.WorkerGlobal().Studio.VideoOverview.SandboxProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "video overview get sandbox provider failed")
	}
	return sandboxservice.New(g.deps.SandboxRepository, mgr, g.deps.DistLock), nil
}

// sandbox 工作目录结构如下：
//
//	/tmp/{userId}/{notebookId}/                 ← 沙箱 WorkspaceDir（真实 vendor）
//	├── vendor/                                 ← 沙箱上传的 vendor（含 gsap.browser.js）
//	└── studiovideooverview/
//	    └── {artifactId}/                       ← 视频逻辑工作区（prompt WorkspaceDir、Bash cwd）
//	        ├── vendor -> ../../vendor          ← 软链，兼容相对路径 vendor/gsap.browser.js
//	        ├── skills/<name>/SKILL.md          ← 标准 Agent Skills（契约/视觉/动画/镜头/数据/转场），只读
//	        ├── components/<name>/<name>.html   ← 可复用动画片段，INDEX.md 为索引，只读
//	        ├── audio/audio_{seg}-{line}.wav
//	        ├── .check/                         ← 逐镜临时校验目录
//	        ├── render.log / ffmpeg.log         ← 渲染/压制日志
//	        ├── index.html
//	        └── output/video.mp4
func ensureVideoOverviewWorkspace(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	vendorLink := path.Join(workspaceDir, "vendor")
	audioDir := path.Join(workspaceDir, "audio")
	outputDir := path.Join(workspaceDir, "output")
	checkDir := path.Join(workspaceDir, ".check")
	skillsDir := path.Join(workspaceDir, "skills")
	cmd := fmt.Sprintf(
		"mkdir -p %s %s %s %s && ln -sfn ../../vendor %s && test -s %s/gsap.browser.js",
		shellQuote(audioDir),
		shellQuote(outputDir),
		shellQuote(checkDir),
		shellQuote(skillsDir),
		shellQuote(vendorLink),
		shellQuote(vendorLink),
	)
	exec, err := sandbox.Run(ctx, sandboxent.Command{Command: cmd})
	if err != nil {
		return errors.WithMessagef(err, "mkdir video workspace failed: %s", workspaceDir)
	}
	if !exec.Success() {
		return errors.Errorf(
			"video workspace not ready (vendor/gsap.browser.js missing or empty?): exit=%d stderr=%s",
			exec.ExitCode, string(exec.Stderr),
		)
	}
	return nil
}

func (g *hyperframesVideoGenerator) syncAudioToSandbox(
	ctx context.Context,
	sandbox sandboxent.Sandbox,
	workspaceDir string,
	meta *audioCheckpointMeta,
) error {
	parts := meta.sortedAudioParts()
	if len(parts) == 0 {
		return errors.New("no audio parts to sync into sandbox")
	}

	limit := conf.WorkerGlobal().Studio.VideoOverview.AudioSynthConcurrency
	if limit <= 0 {
		limit = 20
	}

	gp, gctx := errgroup.WithContext(ctx)
	gp.SetLimit(limit)
	for _, part := range parts {
		p := part
		gp.Go(func() error {
			obj, err := g.deps.ObjectStorage.GetObject(gctx,
				&storage.GetObjectRequest{Key: p.StoreKey},
			)
			if err != nil {
				return errors.WithMessagef(err, "get audio object failed: %s", p.StoreKey)
			}
			dest := audioSandboxPath(workspaceDir, p.SegmentIndex, p.LineIndex)
			if err := sandbox.WriteFile(gctx, dest, bytes.NewReader(obj.Body)); err != nil {
				return errors.WithMessagef(err, "write audio to sandbox failed: %s", dest)
			}
			return nil
		})
	}
	if err := gp.Wait(); err != nil {
		return err
	}

	slog.InfoContext(ctx, "synced video line audio into sandbox",
		slog.String("workspace", workspaceDir),
		slog.Int("count", len(parts)),
		slog.Int("concurrency", limit),
	)
	return nil
}

func formatVideoStoreKey(notebookId, artifactId valobj.Id) string {
	return fmt.Sprintf("artifact/%s/%s.mp4", notebookId.String(), artifactId.String())
}

const checkMP4ValidToolName = "CheckMP4"

type checkMP4ToolInput struct {
	Filename string `json:"filename" jsonschema_description:"title=target file path,description=The target MP4 file path"`
}

func (g *hyperframesVideoGenerator) getCheckMP4ValidTool(sandbox sandboxent.Sandbox) (einotool.InvokableTool, error) {
	tool, err := einotoolutils.InferTool(
		checkMP4ValidToolName,
		"Validate an MP4 file. Input is the filename of the MP4 you just rendered. "+
			"Always call this on your output file before finishing the task. "+
			"Returns 'OK' if the file is a valid MP4; otherwise it returns an error message describing what is wrong, "+
			"and you must fix the composition and render again.",
		func(ctx context.Context, input *checkMP4ToolInput) (output string, err error) {
			if err := checkMP4ArtifactValid(ctx, sandbox, input.Filename); err != nil {
				return "", err
			}
			return "OK", nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer check mp4 tool failed, err=%v", err)
	}
	return tool, nil
}
