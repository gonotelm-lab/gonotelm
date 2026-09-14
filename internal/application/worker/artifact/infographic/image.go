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
	t2ischema "github.com/gonotelm-lab/multimodal/image/schema"
	t2iutil "github.com/gonotelm-lab/multimodal/image/util"
)

const imageDownloadTimeout = 5 * time.Minute

type imageGenerator struct {
	deps           *types.WorkerDeps
	downloadClient *http.Client
}

func newImageGenerator(deps *types.WorkerDeps) *imageGenerator {
	return &imageGenerator{
		deps:           deps,
		downloadClient: httpclient.NewBuilder(nil).WithTimeout(imageDownloadTimeout).Build(),
	}
}

func (g *imageGenerator) generate(
	ctx context.Context,
	artifactId valobj.Id,
	payload *artifactentity.InfoGraphicPayload,
	imagePrompt string,
) (*StorageResult, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioInfoGraphicImageScene)

	cfg := conf.WorkerGlobal().Studio.InfoGraphic

	generator, err := g.deps.Text2Image.GetProvider(cfg.ImageModelProvider)
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
		t2iutil.WithResolveHttpClient(g.downloadClient),
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
	storeKey := formatArtifactStoreKey(payload.NotebookId, artifactId, ext)

	if err := types.UploadReader(ctx, g.deps.ObjectStorage, storeKey, contentType, stream); err != nil {
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
