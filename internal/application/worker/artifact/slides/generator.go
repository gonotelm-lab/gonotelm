package slides

import (
	"context"
	"fmt"
	"log/slog"
	"path"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	sandboxent "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/entity"
	sandboxservice "github.com/gonotelm-lab/gonotelm/internal/domain/sandbox/service"
	sourceentitiy "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	workererrors "github.com/gonotelm-lab/gonotelm/internal/domain/worker/errors"

	"github.com/bytedance/sonic"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einotoolutils "github.com/cloudwego/eino/components/tool/utils"
	einoschema "github.com/cloudwego/eino/schema"
)

const slidesMaxCompensateRetry = 3

type Generator struct {
	deps *types.ServiceDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

type slidesOutlineExpectation struct {
	Title   string `json:"title"`
	Outline string `json:"outline"`
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	sources, err := g.loadOutlineSources(ctx, req.SourceIds)
	if err != nil {
		return nil, errors.WithMessage(err, "load outline sources failed")
	}

	outlineExp, err := g.ensureOutline(ctx, req, sources)
	if err != nil {
		return nil, errors.WithMessage(err, "ensure outline failed")
	}

	slog.DebugContext(ctx, "slides outline generated", slog.String("notebook_id", req.NotebookId.String()),
		slog.String("artifact_id", req.ArtifactId.String()),
	)

	// 保留沙箱等待自然过期
	sandbox, err := g.ensureSandbox(ctx, req)
	if err != nil {
		return nil, errors.WithMessagef(err, "ensure sandbox failed")
	}

	slog.DebugContext(ctx, "slides generation ensure sandbox done")

	result, err := g.generatePPTX(ctx, req, outlineExp, sandbox, sources)
	if err != nil {
		return nil, errors.WithMessage(err, "generate slides failed")
	}

	slog.DebugContext(ctx, "slides generation pptx done")

	resultBytes, err := sonic.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "marshal slides generation result failed")
	}

	return &types.Response{
		Title:      outlineExp.Title,
		Result:     resultBytes,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}

func (g *Generator) llmOptions(jsonObject bool, thinking bool) []einomodel.Option {
	var (
		provider = conf.WorkerGlobal().Studio.Slides.ModelProvider
		model    = conf.WorkerGlobal().Studio.Slides.Model
	)
	opts := []einomodel.Option{
		chat.WithModel(model),
		chat.WithThinking(provider, thinking),
	}
	if jsonObject {
		opts = append(opts, chat.WithResponseJsonObject(provider))
	}
	return opts
}

func (g *Generator) buildAgent(req *types.Request, maxRound int, jsonObject bool, thinking bool) (*types.Agent, error) {
	round := conf.WorkerGlobal().Studio.Slides.MaxRound
	if maxRound > 0 {
		round = maxRound
	}

	return types.BuildSourceExploreAgent(
		g.deps,
		conf.WorkerGlobal().Studio.Slides.ModelProvider,
		conf.WorkerGlobal().Studio.Slides.Model,
		round,
		g.llmOptions(jsonObject, thinking),
		req.NotebookId,
		req.SourceIds,
		true,
	)
}

// ensureOutline 先尝试从 checkpoint 恢复大纲，否则生成并写入 checkpoint。
func (g *Generator) ensureOutline(ctx context.Context, req *types.Request, sources []OutlineSource) (*slidesOutlineExpectation, error) {
	ckpt := g.loadCheckpoint(ctx, req)
	if outline := restoreOutline(ctx, req.ArtifactId, ckpt); outline != nil {
		slog.InfoContext(ctx, "slides generator restore from checkpoint 1", slog.String("artifact_id", req.ArtifactId.String()))
		return outline, nil
	}

	outline, err := g.generateOutline(ctx, req, sources)
	if err != nil {
		return nil, err
	}

	if err := g.saveOutlineCheckpoint(ctx, req.ArtifactId, ckpt, outline); err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "save slides outline checkpoint failed, err=%v", err)
	}

	return outline, nil
}

func (g *Generator) loadCheckpoint(ctx context.Context, req *types.Request) *workerentity.Checkpoint {
	ckpt, err := g.deps.CheckpointRepository.FindByArtifactId(ctx, req.ArtifactId)
	if err != nil {
		if !errors.Is(err, workererrors.ErrCheckpointNotFound) {
			slog.ErrorContext(ctx, "find checkpoint failed",
				slog.String("artifact_id", req.ArtifactId.String()), slog.Any("err", err))
		}
		return nil
	}
	return ckpt
}

func (g *Generator) loadOutlineSources(ctx context.Context, sourceIds []valobj.Id) ([]OutlineSource, error) {
	sources := make([]OutlineSource, 0, len(sourceIds))
	for _, id := range sourceIds {
		entry := OutlineSource{Id: id.String()}
		stat, err := g.deps.Agentize.StatSource(ctx, id)
		if err != nil {
			slog.WarnContext(ctx, "slides outline load source abstract failed",
				slog.String("source_id", id.String()), slog.Any("err", err))
		} else if stat != nil {
			entry.Abstract = stat.Abstract
		}
		sources = append(sources, entry)
	}
	return sources, nil
}

// 探索生成幻灯片大纲
func (g *Generator) generateOutline(ctx context.Context, req *types.Request, sources []OutlineSource) (*slidesOutlineExpectation, error) {
	agent, err := g.buildAgent(req, 0, true, false) // 大纲：关 thinking
	if err != nil {
		return nil, errors.Wrapf(err, "build source explore agent for slides outline failed, err=%v", err)
	}

	payload := artifactentity.PayloadAs[*artifactentity.SlidesPayload](req.Payload)
	msgs, err := RenderSlidesOutline(ctx, sources, payload.GetLanguage(), payload.GetTip())
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate slides outline message failed, err=%v", err)
	}

	output, err := agent.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrap(err, "generate slides outline output failed")
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate slides outline agent usage: %+v", agent.TokenUsage()))

	expect, parseErr := parseAgentOutput(ctx, output.Content)
	if parseErr == nil {
		return expect, nil
	}

	// 失败重试
	lastContent := output.Content
	lastErr := parseErr
	msgs = append([]*einoschema.Message{}, agent.AccumulatedMessages()...)

	for attempt := 1; attempt <= slidesMaxCompensateRetry; attempt++ {
		slog.WarnContext(ctx, "slides outline agent output invalid, compensating",
			slog.String("notebook_id", req.NotebookId.String()),
			slog.Int("attempt", attempt),
			slog.Int("max_retry", slidesMaxCompensateRetry),
			slog.Any("err", lastErr),
			slog.Any("usage", agent.TokenUsage()),
		)

		compensateMsgs := append([]*einoschema.Message{}, msgs...)
		compensateMsgs = append(compensateMsgs, types.BuildCompensateMessage(lastContent, slidesOutlineCompensateRules(lastErr)))

		llmResp, genErr := agent.BaseLLM().Generate(ctx, compensateMsgs, g.llmOptions(true, false)...)
		if genErr != nil {
			return nil, errors.Wrapf(errors.ErrLLM,
				"slides outline compensate generate failed on attempt %d, err=%v",
				attempt,
				genErr,
			)
		}

		expect, parseErr = parseAgentOutput(ctx, llmResp.Content)
		if parseErr == nil {
			return expect, nil
		}

		lastContent = llmResp.Content
		lastErr = parseErr
		msgs = append(compensateMsgs, &einoschema.Message{
			Role:    einoschema.Assistant,
			Content: lastContent,
		})
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"slides outline agent output invalid after %d retries, err=%v",
		slidesMaxCompensateRetry,
		lastErr,
	)
}

func slidesOutlineCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `outline`",
		"`title` is the PPT title, preferably 10-30 characters",
		"`outline` is a Markdown outline string",
	}
	if validateErr != nil {
		rules = append(rules, "Error: "+validateErr.Error())
	}
	return rules
}

func parseAgentOutput(ctx context.Context, content string) (*slidesOutlineExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect slidesOutlineExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "slides outline direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "slides outline output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	if expect.Title == "" {
		return nil, fmt.Errorf("slides outline title is empty")
	}
	if expect.Outline == "" {
		return nil, fmt.Errorf("slides outline outline is empty")
	}

	return &expect, nil
}

func restoreOutline(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *slidesOutlineExpectation {
	if ckpt == nil || ckpt.Field1 == nil {
		return nil
	}
	var outline slidesOutlineExpectation
	if err := sonic.Unmarshal(ckpt.Field1, &outline); err != nil {
		slog.WarnContext(ctx, "unmarshal slides outline failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}

	return &outline
}

// step1 field1
func (g *Generator) saveOutlineCheckpoint(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	outline *slidesOutlineExpectation,
) error {
	data, err := sonic.Marshal(outline)
	if err != nil {
		return err
	}
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField1(data)

	return g.deps.CheckpointRepository.Save(ctx, ckpt)
}

// 准备好沙箱
func (g *Generator) ensureSandbox(ctx context.Context, req *types.Request) (sandboxent.Sandbox, error) {
	ss, err := g.getSandboxSerivce()
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

// ensureSlidesWorkspace 在沙箱 notebook 目录下创建 artifact 子目录，并挂上 vendor 软链。
//
// sandbox 工作目录结构如下：
//
//	/tmp/{userId}/{notebookId}/           ← 沙箱 WorkspaceDir（Bash 默认 cwd、真实 vendor）
//	├── vendor/                           ← opensandbox 上传的 pptxgenjs
//	└── {artifactId}/                     ← slides 逻辑工作区（prompt WorkspaceDir）
//	    ├── vendor -> ../vendor           ← 软链，兼容 slides/../vendor 的 require
//	    └── slides/
//	        ├── slide-01.js
//	        ├── compile.js
//	        └── output/presentation.pptx
func ensureSlidesWorkspace(ctx context.Context, sandbox sandboxent.Sandbox, workspaceDir string) error {
	vendorLink := path.Join(workspaceDir, "vendor")
	slidesOut := path.Join(workspaceDir, "slides", "output")
	// 建工作区 + vendor 软链；并校验 notebook 级 vendor/standalone.cjs 非空（0 字节会导致 agent 全盘找库）
	cmd := fmt.Sprintf(
		"mkdir -p %s && ln -sfn ../vendor %s && test -s %s/standalone.cjs",
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

func (g *Generator) getSandboxSerivce() (*sandboxservice.Service, error) {
	mgr, err := g.deps.Sandbox.GetManager(conf.WorkerGlobal().Studio.Slides.SandboxProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "slides worker get provider failed")
	}

	service := sandboxservice.New(g.deps.SandboxRepository, mgr, g.deps.DistLock)
	return service, nil
}

type checkPPTXToolInput struct {
	Filename string `json:"filename" jsonschema_description:"title=targe file path,description=The target file path"`
}

func (g *Generator) generatePPTX(
	ctx context.Context,
	req *types.Request,
	outlineExp *slidesOutlineExpectation,
	sandbox sandboxent.Sandbox,
	sources []OutlineSource,
) (*SlidesStorageResult, error) {
	// use thinking in pptx generation, it will take much longer
	agent, err := g.buildAgent(req, conf.WorkerGlobal().Studio.Slides.GenerateMaxRound, false, true)
	if err != nil {
		return nil, errors.WithMessage(err, "build generate pptx agent failed")
	}

	checkPPTXTool, err := einotoolutils.InferTool(
		"CheckPPTX",
		"Validate a PPTX file. Input is the filename of the PPTX you just generated. "+
			"Always call this on your output file before finishing the task. "+
			"Returns 'OK' if the file is a valid PPTX; otherwise it returns an error message describing what is wrong, "+
			"and you must fix the file and validate again.",
		func(ctx context.Context, input *checkPPTXToolInput) (output string, err error) {
			if err := g.checkPPTXArtifactValid(ctx, sandbox, input.Filename); err != nil {
				return "", err
			}
			return "OK", nil
		},
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "infer check pptx tool failed, err=%v", err)
	}

	// 额外绑定沙箱工具
	err = agent.AppendTools(map[string]einotool.InvokableTool{
		tools.BashToolName:      tools.NewBashTool(sandbox),
		tools.ReadFileToolName:  tools.NewReadFileTool(sandbox),
		tools.WriteFileToolName: tools.NewWriteFileTool(sandbox),
		tools.EditFileToolName:  tools.NewEditFileTool(sandbox),
		tools.ListDirToolName:   tools.NewListDirTool(sandbox),
		"CheckPPTX":             checkPPTXTool,
	})
	if err != nil {
		return nil, errors.Wrap(err, "gen pptx agent append tools failed")
	}

	sandboxDesc := sandbox.Description()
	// slides 工作区在沙箱 Workspace 下多一层 artifactId，隔离同 notebook 多 artifact
	workspaceDir := path.Join(sandboxDesc.Key.WorkspaceDir(), req.ArtifactId.String())
	if err := ensureSlidesWorkspace(ctx, sandbox, workspaceDir); err != nil {
		return nil, err
	}
	outputLocation := path.Join(workspaceDir, "slides", "output", "presentation.pptx")
	payload := artifactentity.PayloadAs[*artifactentity.SlidesPayload](req.Payload)
	msgs, err := RenderSlides(ctx,
		outlineExp.Title, outlineExp.Outline,
		sources,
		sandboxDesc.Runtime, workspaceDir,
		outputLocation,
		payload.GetVisualStyle(),
		payload.GetLanguage(),
		payload.GetTip(),
	)
	if err != nil {
		return nil, errors.Wrap(err, "gen pptx render prompts failed")
	}

	_, err = agent.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrap(err, "generate pptx output failed")
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate pptx output agent usage: %+v", agent.TokenUsage()))

	// 前往沙箱检查
	pptxReader, err := sandbox.ReadFile2(ctx, outputLocation)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := pptxReader.Close(); cerr != nil {
			slog.WarnContext(ctx, "slides generator pptx reader close failed", slog.Any("err", cerr))
		}
	}()

	storeKey := formatSlidesStoreKey(req.NotebookId, req.ArtifactId)
	if err := types.UploadReader(ctx, g.deps.ObjectStorage, storeKey, sourceentitiy.MimeTypePPTX, pptxReader); err != nil {
		return nil, errors.Wrapf(err, "upload slides object failed, artifact_id=%s", req.ArtifactId)
	}

	return &SlidesStorageResult{
		StoreKey:    storeKey,
		ContentType: sourceentitiy.MimeTypePPTX,
	}, nil
}

func formatSlidesStoreKey(notebookId, artifactId valobj.Id) string {
	return fmt.Sprintf("artifact/%s/%s.pptx", notebookId.String(), artifactId.String())
}
