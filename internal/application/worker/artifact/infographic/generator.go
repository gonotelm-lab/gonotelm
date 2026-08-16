package infographic

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	workererrors "github.com/gonotelm-lab/gonotelm/internal/domain/worker/errors"
	t2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	t2iutil "github.com/gonotelm-lab/multimodal/image/util"
)

type Generator struct {
	deps           *types.ServiceDeps
	downloadClient *http.Client
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
	return &Generator{
		deps:           deps,
		downloadClient: httpclient.NewBuilder(nil).WithTimeout(5 * time.Minute).Build(),
	}
}

type infoGraphicExpectation struct {
	Title       string `json:"title"`
	ImagePrompt string `json:"image_prompt"`
}

func (ig *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	payload := artifactentity.PayloadAs[*artifactentity.InfoGraphicPayload](req.Payload)

	expect, storageResult, err := ig.generate(ctx, req.ArtifactId, payload)
	if err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(storageResult)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal infographic storage result err=%v", err)
	}

	// 暂时保留 checkpoint 审计
	// if err = ig.deps.CheckpointRepository.DeleteByArtifactId(ctx, req.ArtifactId); err != nil {
	// 	slog.ErrorContext(ctx, "delete checkpoint failed", slog.String("artifact_id", req.ArtifactId.String()), slog.Any("err", err))
	// }

	return &types.Response{
		Title:      expect.Title,
		Result:     result,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}

func (ig *Generator) generate(
	ctx context.Context,
	artifactId valobj.Id,
	payload *artifactentity.InfoGraphicPayload,
) (*infoGraphicExpectation, *StorageResult, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioInfographicScene)

	ckpt, err := ig.deps.CheckpointRepository.FindByArtifactId(ctx, artifactId)
	if err != nil {
		if !errors.Is(err, workererrors.ErrCheckpointNotFound) {
			slog.ErrorContext(ctx, "find checkpoint failed", slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		}
	}

	var expect *infoGraphicExpectation
	if ckpt != nil && ckpt.Field1 != nil {
		if err := sonic.Unmarshal(ckpt.Field1, &expect); err != nil {
			slog.WarnContext(ctx, "unmarshal checkpoint prompt failed", slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		}
	}

	if expect == nil {
		expect, err = ig.generateImagePrompt(ctx, payload)
		if err != nil {
			return nil, nil, err
		}

		promptBytes, _ := sonic.Marshal(expect)
		if ckpt == nil {
			ckpt = workerentity.NewCheckpoint(artifactId)
		}
		ckpt.UpdateField1(promptBytes)
		if err := ig.deps.CheckpointRepository.Save(ctx, ckpt); err != nil {
			slog.WarnContext(ctx, "save checkpoint failed", slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		}
	}

	slog.DebugContext(ctx, "generate infographic expectation done, now generate image",
		slog.String("task_id", artifactId.String()),
		slog.String("title", expect.Title),
	)

	storageResult, err := ig.generateAndStoreImage(ctx, artifactId, payload, expect.ImagePrompt)
	if err != nil {
		return nil, nil, err
	}

	return expect, storageResult, nil
}

func (ig *Generator) generateImagePrompt(
	ctx context.Context,
	payload *artifactentity.InfoGraphicPayload,
) (*infoGraphicExpectation, error) {
	cfg := conf.WorkerGlobal().Studio.InfoGraphic
	modelOption := chat.WithModel(cfg.Model)

	bindAllTools := payload.DetailLevel != artifactentity.InfoGraphicDetailLevelConcise

	ag, err := types.BuildSourceExploreAgent(
		ig.deps,
		cfg.ModelProvider,
		cfg.Model,
		cfg.MaxRound,
		[]einomodel.Option{modelOption},
		payload.NotebookId,
		payload.SourceIds,
		bindAllTools,
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to build source explore agent for infographic")
	}

	sourceIds := types.SourceIDsToStrings(payload.SourceIds)
	vars := TemplateVars{
		SourceIds:    sourceIds,
		TextLanguage: payload.TextLanguage,
		ExtraPrompt:  payload.ExtraPrompt,
		Orientation:  payload.Orientation,
		DetailLevel:  payload.DetailLevel,
		VisualStyle:  payload.VisualStyle,
	}
	msgs, err := RenderInfographic(ctx, vars)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "render infographic prompt failed, err=%v", err)
	}

	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate infographic prompt failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate infographic agent usage: %+v", ag.TokenUsage()))

	expect, err := ig.parseAgentOutput(ctx, output.Content)
	if err == nil {
		return expect, nil
	}

	slog.WarnContext(ctx, "infographic agent output invalid, compensating",
		slog.String("notebook_id", payload.NotebookId.String()),
		slog.String("output", output.Content),
		slog.Any("usage", ag.TokenUsage()),
		slog.Any("err", err),
	)

	msgs = append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	msgs = append(msgs, types.BuildCompensateMessage(output.Content, []string{
		"JSON 字段必须且仅能包含 title 和 image_prompt",
		"title 长度必须为 10-30 字",
		"image_prompt 必须为完整文生图 prompt 字符串",
	}))

	llmResp, genErr := ag.BaseLLM().Generate(ctx, msgs, modelOption)
	if genErr != nil {
		return nil, errors.Wrapf(errors.ErrLLM,
			"infographic compensate generate failed, err=%v",
			genErr,
		)
	}

	expect, err = ig.parseAgentOutput(ctx, llmResp.Content)
	if err == nil {
		return expect, nil
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"infographic agent output invalid after compensation, first_output=%q, compensate_output=%q, err=%v",
		output.Content,
		llmResp.Content,
		err,
	)
}

func (ig *Generator) parseAgentOutput(
	ctx context.Context,
	content string,
) (*infoGraphicExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect infoGraphicExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"infographic direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", content),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "infographic output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", content))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	expect.ImagePrompt = strings.TrimSpace(expect.ImagePrompt)
	if expect.ImagePrompt == "" {
		return nil, fmt.Errorf("image_prompt is empty")
	}

	return &expect, nil
}

func (ig *Generator) generateAndStoreImage(
	ctx context.Context,
	artifactId valobj.Id,
	payload *artifactentity.InfoGraphicPayload,
	imagePrompt string,
) (*StorageResult, error) {
	cfg := conf.WorkerGlobal().Studio.InfoGraphic

	generator, err := ig.deps.Text2Image.GetProvider(cfg.ImageModelProvider)
	if err != nil {
		return nil, errors.WithMessagef(err, "get text2image provider failed")
	}

	w, h := payload.Orientation.ImageSize()
	resp, err := generator.Generate(ctx,
		&t2ischema.Request{
			Model:  cfg.ImageModel,
			Prompt: imagePrompt,
			Size:   fmt.Sprintf("%dx%d", w, h),
		})
	if err != nil {
		return nil, errors.Wrapf(err, "text2image generate failed")
	}

	imageReader, err := t2iutil.ResolveResponse(resp,
		t2iutil.WithResolveContext(ctx),
		t2iutil.WithResolveHttpClient(ig.downloadClient),
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "resolve generated image failed")
	}
	defer imageReader.Close()

	var header bytes.Buffer
	// imageReader中消费的前缀同时加载到header上
	mimeType, err := mimetype.DetectReader(io.TeeReader(imageReader, &header))
	if err != nil {
		return nil, errors.WithMessagef(err, "detect generated image mime failed")
	}
	stream := io.MultiReader(bytes.NewReader(header.Bytes()), imageReader) // 剩余的imageReader

	ext := mimeType.Extension()
	contentType := mimeType.String()
	storeKey := formatArtifactStoreKey(payload.NotebookId, artifactId, ext)

	if err := types.UploadReader(ctx, ig.deps.ObjectStorage, storeKey, contentType, stream); err != nil {
		return nil, errors.WithMessagef(err, "upload infographic image failed")
	}

	width, height := decodeImageConfigOrIgnore(header.Bytes())

	return &StorageResult{
		StoreKey:    storeKey,
		ContentType: contentType,
		Image: &StorageResultImage{
			Width:  width,
			Height: height,
		},
	}, nil
}

func formatArtifactStoreKey(notebookId, artifactId valobj.Id, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	return fmt.Sprintf("artifact/%s/%s%s", notebookId.String(), artifactId.String(), ext)
}

func decodeImageConfigOrIgnore(imageData []byte) (width, height int) {
	c, _, err := image.DecodeConfig(bytes.NewReader(imageData))
	if err == nil {
		return c.Width, c.Height
	}

	return
}
