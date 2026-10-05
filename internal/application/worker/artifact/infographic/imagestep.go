package infographic

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
	t2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	t2iutil "github.com/gonotelm-lab/multimodal/image/util"
)

const imageDownloadTimeout = 5 * time.Minute

// imageStep 依据文生图 prompt 生成图片并上传。
type imageStep struct {
	deps           *types.WorkerDeps
	downloadClient *http.Client
}

func newImageStep(deps *types.WorkerDeps) *imageStep {
	return &imageStep{
		deps:           deps,
		downloadClient: httpclient.NewBuilder(nil).WithTimeout(imageDownloadTimeout).Build(),
	}
}

func (s *imageStep) Name() string { return "image" }

func (s *imageStep) Execute(ctx context.Context, data *pipeline.Data) error {
	req := types.RequestFrom(data)
	result, err := s.generate(ctx, req.ArtifactId, payloadFrom(req), expectationFrom(data).ImagePrompt)
	if err != nil {
		return err
	}
	data.Set(dataKeyResult, result)
	return nil
}

func (s *imageStep) generate(
	ctx context.Context,
	artifactId valobj.Id,
	payload *artifactentity.InfoGraphicPayload,
	imagePrompt string,
) (*StorageResult, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioInfoGraphicImageScene)

	cfg := conf.WorkerGlobal().Studio.InfoGraphic

	generator, err := s.deps.Text2Image.GetProvider(cfg.ImageModelProvider)
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
		t2iutil.WithResolveHttpClient(s.downloadClient),
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
	stream := io.MultiReader(bytes.NewReader(header.Bytes()), imageReader)

	ext := mimeType.Extension()
	contentType := mimeType.String()
	storeKey, err := s.deps.KeyFactory.New(artifactImagePath(payload.NotebookId, artifactId, ext), false)
	if err != nil {
		return nil, errors.WithMessage(err, "create infographic image store key failed")
	}

	if err := s.deps.ObjectStorage.UploadReader(ctx, storeKey, contentType, stream); err != nil {
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

func artifactImagePath(notebookId, artifactId valobj.Id, ext string) string {
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
