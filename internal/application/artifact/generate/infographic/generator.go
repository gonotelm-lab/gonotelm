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
	"strings"

	"github.com/gabriel-vasile/mimetype"

	generatetypes "github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	t2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	t2iutil "github.com/gonotelm-lab/multimodal/image/util"
)

const MaxArtifactTitleLength = 128

type Generator struct {
	deps *generatetypes.ServiceDeps
}

var _ generatetypes.Generator = &Generator{}

func New(deps *generatetypes.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

type infographicExpectation struct {
	Title       string `json:"title"`
	ImagePrompt string `json:"image_prompt"`
}

func (ig *Generator) Generate(ctx context.Context, req *generatetypes.Request) (*generatetypes.Response, error) {
	payload, ok := req.Payload.(*artifactentity.InfoGraphicPayload)
	if !ok {
		return nil, errors.ErrParams.Msgf("infographic generator expects InfoGraphicPayload")
	}

	expect, storageResult, err := ig.generate(ctx, req.ArtifactId, payload)
	if err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(storageResult)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal infographic storage result err=%v", err)
	}

	return &generatetypes.Response{
		Title:      expect.Title,
		Result:     result,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}

func (ig *Generator) generate(
	ctx context.Context,
	taskId valobj.Id,
	payload *artifactentity.InfoGraphicPayload,
) (*infographicExpectation, *StorageResult, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioInfographicScene)

	expect, err := ig.generateImagePrompt(ctx, payload)
	if err != nil {
		return nil, nil, err
	}

	slog.DebugContext(ctx, "generate infographic expectation done, now generate image",
		slog.String("task_id", taskId.String()),
		slog.String("title", expect.Title),
	)

	storageResult, err := ig.generateAndStoreImage(ctx, taskId, payload, expect.ImagePrompt)
	if err != nil {
		return nil, nil, err
	}

	return expect, storageResult, nil
}

func (ig *Generator) generateImagePrompt(
	ctx context.Context,
	payload *artifactentity.InfoGraphicPayload,
) (*infographicExpectation, error) {
	cfg := conf.Global().Studio.InfoGraphic
	modelOption := chat.WithModel(cfg.Model)

	bindAllTools := payload.DetailLevel != artifactentity.ArtifactInfoGraphicDetailLevelConcise

	ag, err := generatetypes.BuildSourceExploreAgent(
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

	sourceIds := generatetypes.SourceIDsToStrings(payload.SourceIds)
	vars := TemplateVars{
		SourceIds:    sourceIds,
		TextLanguage: payload.TextLanguage,
		ExtraPrompt:  payload.ExtraPrompt,
		Orientation:  payload.Orientation,
		DetailLevel:  payload.DetailLevel,
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
	msgs = append(msgs, &einoschema.Message{
		Role: einoschema.User,
		Content: fmt.Sprintf(
			"你刚才输出的结果不符合要求，请严格重输。\n当前输出：\n%s\n\n"+
				"要求：\n"+
				"1) 只输出一个合法 JSON 对象，不要任何解释性文字\n"+
				"2) JSON 字段必须且仅能包含 title 和 image_prompt\n"+
				"3) title 长度必须为 10-30 字\n"+
				"4) image_prompt 必须为完整文生图 prompt 字符串\n"+
				"5) 不允许输出 ```json 代码块包裹",
			output.Content,
		),
	})

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
) (*infographicExpectation, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect infographicExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.WarnContext(ctx,
				"infographic direct output unmarshal failed, fallback to extracted json candidates",
				slog.Any("err", err),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		return nil, err
	}

	expect.Title = strings.TrimSpace(expect.Title)
	expect.ImagePrompt = strings.TrimSpace(expect.ImagePrompt)
	expect.Title = pkgstring.TruncateRune(expect.Title, MaxArtifactTitleLength)
	if expect.ImagePrompt == "" {
		return nil, fmt.Errorf("image_prompt is empty")
	}

	return &expect, nil
}

func (ig *Generator) generateAndStoreImage(
	ctx context.Context,
	taskId valobj.Id,
	payload *artifactentity.InfoGraphicPayload,
	imagePrompt string,
) (*StorageResult, error) {
	cfg := conf.Global().Studio.InfoGraphic

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
		return nil, errors.Wrapf(errors.ErrInner, "text2image generate failed, err=%v", err)
	}

	imageReader, err := t2iutil.ResolveResponse(resp, t2iutil.WithResolveContext(ctx))
	if err != nil {
		return nil, errors.WithMessagef(err, "resolve generated image failed")
	}
	defer imageReader.Close()

	imageData, err := io.ReadAll(imageReader)
	if err != nil {
		return nil, errors.WithMessagef(err, "read generated image failed")
	}

	mimeType := mimetype.Detect(imageData)
	ext := mimeType.Extension()
	contentType := mimeType.String()
	storeKey := formatArtifactStoreKey(payload.NotebookId, taskId, ext)
	err = ig.deps.ObjectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         storeKey,
		Body:        imageData,
		ContentType: contentType,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "upload infographic image failed")
	}

	width, height := decodeImageConfigOrIgnore(imageData)

	return &StorageResult{
		StoreKey:    storeKey,
		ContentType: contentType,
		Image: &StorageResultImage{
			Width:  width,
			Height: height,
		},
	}, nil
}

func formatArtifactStoreKey(notebookId, taskId valobj.Id, ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}

	return fmt.Sprintf("artifact/%s/%s%s", notebookId.String(), taskId.String(), ext)
}

func decodeImageConfigOrIgnore(imageData []byte) (width, height int) {
	c, _, err := image.DecodeConfig(bytes.NewReader(imageData))
	if err == nil {
		return c.Width, c.Height
	}

	return
}
