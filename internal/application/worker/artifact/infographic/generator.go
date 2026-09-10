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
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	workererrors "github.com/gonotelm-lab/gonotelm/internal/domain/worker/errors"
	t2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	t2iutil "github.com/gonotelm-lab/multimodal/image/util"
)

type Generator struct {
	deps           *types.WorkerDeps
	downloadClient *http.Client
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
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

	expect, storageResult, err := ig.generate(ctx, req, payload)
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
	req *types.Request,
	payload *artifactentity.InfoGraphicPayload,
) (*infoGraphicExpectation, *StorageResult, error) {
	artifactId := req.ArtifactId

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
		expect, err = ig.generateImagePrompt(ctx, req, payload)
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
	req *types.Request,
	payload *artifactentity.InfoGraphicPayload,
) (*infoGraphicExpectation, error) {
	vars := TemplateVars{
		SourceIds:    types.SourceIDsToStrings(req.SourceIds),
		TextLanguage: payload.TextLanguage,
		ExtraPrompt:  payload.ExtraPrompt,
		Orientation:  payload.Orientation,
		DetailLevel:  payload.DetailLevel,
		VisualStyle:  payload.VisualStyle,
	}
	msgs, err := RenderInfographic(ctx, vars)
	if err != nil {
		return nil, errors.WithMessagef(err, "render infographic prompt failed")
	}

	ag, err := newInfoGraphicAgent(ig.deps, req, payload.DetailLevel != artifactentity.InfoGraphicDetailLevelConcise)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*infoGraphicExpectation](ag, "infographic").
		WithParse(ig.parseAgentOutput).
		WithRules(infoGraphicCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func infoGraphicCompensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `image_prompt`",
		"`title` length must be 10-30 characters",
		"`image_prompt` must be a complete text-to-image prompt string",
	}
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
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "infographic output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
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
