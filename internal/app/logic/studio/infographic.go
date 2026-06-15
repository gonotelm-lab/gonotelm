package studio

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
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	t2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	t2iutil "github.com/gonotelm-lab/multimodal/image/util"
)

type infographicGenerator struct {
	l *Logic
}

var (
	_ taskHandler       = &infographicGenerator{}
	_ iCommonTaskParams = &generateInfographicTaskParams{}
)

type generateInfographicTaskParams struct {
	*commonTaskParams
	*InfoGraphicExtrasParams
}

type InfoGraphicExtrasParams struct {
	ExtraPrompt  string                               `json:"extra_prompt"`
	TextLanguage string                               `json:"text_language"`
	Orientation  model.ArtifactInfoGraphicOrientation `json:"orientation"`
	DetailLevel  model.ArtifactInfoGraphicDetailLevel `json:"detail_level"`
}

func (p *InfoGraphicExtrasParams) GetExtraPrompt() string {
	if p != nil {
		return p.ExtraPrompt
	}

	return ""
}

func (p *InfoGraphicExtrasParams) GetTextLanguage() string {
	if p != nil {
		return p.TextLanguage
	}

	return "zh-cn(简体中文)"
}

func (p *InfoGraphicExtrasParams) GetOrientation() model.ArtifactInfoGraphicOrientation {
	if p != nil {
		return p.Orientation
	}

	return model.ArtifactInfoGraphicOrientationLandscape
}

func (p *InfoGraphicExtrasParams) GetDetailLevel() model.ArtifactInfoGraphicDetailLevel {
	if p != nil {
		return p.DetailLevel
	}

	return model.ArtifactInfoGraphicDetailLevelStandard
}

func (ig *infographicGenerator) handle(
	ctx context.Context,
	task *model.ArtifactTask,
) (*taskHandleResult, error) {
	var params generateInfographicTaskParams
	err := sonic.Unmarshal(task.Payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal generate infographic task params err=%v", err)
	}

	expect, storageResult, err := ig.generate(ctx, task.Id, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate infographic failed, err=%v", err)
	}

	result, err := sonic.Marshal(storageResult)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal infographic storage result err=%v", err)
	}

	slog.InfoContext(ctx, "generate infographic completed",
		slog.String("task_id", task.Id.String()),
		slog.String("store_key", storageResult.StoreKey),
	)

	return &taskHandleResult{
		result:     result,
		resultKind: model.ArtifactResultKindStorage,
		title:      expect.Title,
	}, nil
}

type infographicExpectation struct {
	Title       string `json:"title"`
	ImagePrompt string `json:"image_prompt"`
}

// 生成信息图
//
// step: 1. 使用 LLM 生成文生图的 prompt; 2. 使用文生图模型生成一张图片
func (ig *infographicGenerator) generate(
	ctx context.Context,
	taskID uuid.UUID,
	params *generateInfographicTaskParams,
) (*infographicExpectation, *model.ArtifactStorageResult, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioInfographicScene)

	expect, err := ig.generateImagePrompt(ctx, params)
	if err != nil {
		return nil, nil, err
	}

	slog.DebugContext(ctx, "generate infographic expectation done, now generate image",
		slog.String("task_id", taskID.String()),
		slog.String("title", expect.Title),
	)

	storageResult, err := ig.generateAndStoreImage(
		ctx, taskID, params, expect.ImagePrompt,
	)
	if err != nil {
		return nil, nil, err
	}

	return expect, storageResult, nil
}

func (ig *infographicGenerator) generateImagePrompt(
	ctx context.Context,
	params *generateInfographicTaskParams,
) (*infographicExpectation, error) {
	cfg := conf.Global().Logic.Studio.InfoGraphic
	modelOption := llmchat.WithModel(cfg.Model)

	bindAllTools := params.DetailLevel != model.ArtifactInfoGraphicDetailLevelConcise

	ag, err := ig.l.buildSourceExploreAgent(
		cfg.ModelProvider,
		cfg.Model,
		cfg.MaxRound,
		[]einomodel.Option{modelOption},
		params,
		bindAllTools,
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "failed to build source explore agent for infographic")
	}

	msgs, err := prompts.RenderStudioInfoGraphicMessage(ctx,
		prompts.StudioInfoGraphicTemplateVars{
			SourceIds:    sourceIDsToStrings(params.SourceIds),
			TextLanguage: params.GetTextLanguage(),
			ExtraPrompt:  params.GetExtraPrompt(),
			Orientation:  params.GetOrientation().String(),
			DetailLevel:  params.GetDetailLevel().String(),
		}, pkgcontext.GetLang(ctx))
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
		slog.String("notebook_id", params.getNotebookId().String()),
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

func (ig *infographicGenerator) parseAgentOutput(
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
	expect.Title = pkgstring.TruncateRune(expect.Title, constants.MaxArtifactTitleLength)
	if expect.ImagePrompt == "" {
		return nil, fmt.Errorf("image_prompt is empty")
	}

	return &expect, nil
}

func (ig *infographicGenerator) generateAndStoreImage(
	ctx context.Context,
	taskID uuid.UUID,
	params *generateInfographicTaskParams,
	imagePrompt string,
) (*model.ArtifactStorageResult, error) {
	cfg := conf.Global().Logic.Studio.InfoGraphic

	generator, err := ig.l.text2imageGateway.GetProvider(cfg.ImageModelProvider)
	if err != nil {
		return nil, errors.WithMessagef(err, "get text2image provider failed")
	}

	resp, err := generator.Generate(ctx,
		&t2ischema.Request{
			Model:  cfg.ImageModel,
			Prompt: imagePrompt,
			Size:   params.InfoGraphicExtrasParams.GetOrientation().ImageSize(),
		})
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "text2image generate failed, err=%v", err)
	}

	// TODO trace 时注入自己的http client
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
	storeKey := formatArtifactStoreKey(params.getNotebookId(), taskID, ext)
	err = ig.l.objectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         storeKey,
		Body:        imageData,
		ContentType: contentType,
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "upload infographic image failed")
	}

	width, height := decodeImageConfigOrIgnore(imageData)

	return &model.ArtifactStorageResult{
		StoreKey:    storeKey,
		ContentType: contentType,
		Image: &model.ArtifactStorageResultImage{
			Width:  width,
			Height: height,
		},
	}, nil
}

func decodeImageConfigOrIgnore(imageData []byte) (width, height int) {
	c, _, err := image.DecodeConfig(bytes.NewReader(imageData))
	if err == nil {
		return c.Width, c.Height
	}

	return
}
