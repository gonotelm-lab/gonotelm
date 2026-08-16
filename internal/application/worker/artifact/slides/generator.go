package slides

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
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
	outlineExp, err := g.ensureOutline(ctx, req)
	if err != nil {
		return nil, errors.WithMessage(err, "ensure outline failed")
	}

	sandbox, err := g.ensureSandbox(ctx, req)
	if err != nil {
		return nil, errors.WithMessagef(err, "ensure sandbox failed")
	}

	result, err := g.generatePPTX(ctx, req, outlineExp, sandbox)
	if err != nil {
		return nil, errors.WithMessage(err, "generate slides failed")
	}

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

func (g *Generator) llmOptions() []einomodel.Option {
	var (
		provider = conf.WorkerGlobal().Studio.Slides.ModelProvider
		model    = conf.WorkerGlobal().Studio.Slides.Model
	)
	return []einomodel.Option{
		chat.WithModel(model),
		chat.WithResponseJsonObject(provider),
		chat.WithThinking(provider, false),
	}
}

func (g *Generator) buildAgent(req *types.Request, maxRound int) (*types.Agent, error) {
	round := conf.WorkerGlobal().Studio.Slides.MaxRound
	if maxRound > 0 {
		round = maxRound
	}

	return types.BuildSourceExploreAgent(
		g.deps,
		conf.WorkerGlobal().Studio.Slides.ModelProvider,
		conf.WorkerGlobal().Studio.Slides.Model,
		round,
		g.llmOptions(),
		req.NotebookId,
		req.SourceIds,
		true,
	)
}

// ensureOutline 先尝试从 checkpoint 恢复大纲，否则生成并写入 checkpoint。
func (g *Generator) ensureOutline(ctx context.Context, req *types.Request) (*slidesOutlineExpectation, error) {
	ckpt := g.loadCheckpoint(ctx, req)
	if outline := restoreOutline(ctx, req.ArtifactId, ckpt); outline != nil {
		return outline, nil
	}

	outline, err := g.generateOutline(ctx, req)
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

// 探索生成幻灯片大纲
func (g *Generator) generateOutline(ctx context.Context, req *types.Request) (*slidesOutlineExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioSlidesScene)

	agent, err := g.buildAgent(req, 0)
	if err != nil {
		return nil, errors.Wrapf(err, "build source explore agent for slides outline failed, err=%v", err)
	}

	tip := artifactentity.PayloadAs[*artifactentity.SlidesPayload](req.Payload).GetTip()
	msgs, err := RenderSlidesOutline(ctx, types.SourceIDsToStrings(req.SourceIds), tip)
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

		llmResp, genErr := agent.BaseLLM().Generate(ctx, compensateMsgs, g.llmOptions()...)
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
		"slides outline agent output invalid after %d retries, last_output=%q, err=%v",
		slidesMaxCompensateRetry,
		lastContent,
		lastErr,
	)
}

func slidesOutlineCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON 字段必须且仅能包含 title 和 outline",
		"title 为 PPT 标题，长度建议 10-30 字",
		"outline 为 Markdown 格式大纲字符串",
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
				slog.String("raw_content", content))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "slides outline output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", content))
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

func (g *Generator) getSandboxSerivce() (*sandboxservice.Service, error) {
	mgr, err := g.deps.Sandbox.GetManager(conf.WorkerGlobal().Studio.Slides.SandboxProvider)
	if err != nil {
		return nil, errors.WithMessage(err, "slides worker get provider failed")
	}

	service := sandboxservice.New(g.deps.SandboxRepository, mgr)
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
) (*SlidesStorageResult, error) {
	agent, err := g.buildAgent(req, conf.WorkerGlobal().Studio.Slides.GenerateMaxRound)
	if err != nil {
		return nil, errors.WithMessage(err, "build generate pptx agent failed")
	}

	checkPPTXTool, err := einotoolutils.InferTool(
		"CheckPPTX",
		"Use this tool to check a target file is a valid pptx format file. Input is the target filename",
		func(ctx context.Context, input *checkPPTXToolInput) (output string, err error) {
			if content, err := g.checkPPTXArtifactValid(ctx, sandbox, input.Filename); err != nil {
				return "", err
			} else {
				return content, nil
			}
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
	outputLocation := path.Join(sandboxDesc.Key.WorkspaceDir(), "slides", "output", "presentation.pptx")
	msgs, err := RenderSlides(ctx,
		outlineExp.Title, outlineExp.Outline,
		types.SourceIDsToStrings(req.SourceIds),
		sandboxDesc.Runtime, sandboxDesc.Key.WorkspaceDir(),
		outputLocation,
		artifactentity.PayloadAs[*artifactentity.SlidesPayload](req.Payload).GetTip(),
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
	if err == nil {
		// pptxReader -> pw -->->-> pr (BodyReader)
		pr, pw := io.Pipe()
		errChan := make(chan error, 1)
		go func() {
			defer pw.Close()

			_, err := io.Copy(pw, pptxReader) //
			errChan <- err
		}()

		// 确认存在了 存到Storage中
		storeKey := formatSlidesStoreKey(req.NotebookId, req.ArtifactId)
		err = g.deps.ObjectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
			Key:         storeKey,
			BodyReader:  pr,
			ContentType: sourceentitiy.MimeTypePPTX,
		})
		if copyErr := <-errChan; copyErr != nil {
			return nil, errors.Wrapf(copyErr, "copy err during uploading object, artifact_id=%s", req.ArtifactId)
		}

		if err != nil {
			return nil, errors.Wrapf(err, "upload obejct failed, key=%s", storeKey)
		}
		pptxReader.Close()

		return &SlidesStorageResult{StoreKey: storeKey, ContentType: sourceentitiy.MimeTypePPTX}, nil
	}

	return nil, err
}

func formatSlidesStoreKey(notebookId, artifactId valobj.Id) string {
	return fmt.Sprintf("artifact/%s/%s.pptx", notebookId.String(), artifactId.String())
}
