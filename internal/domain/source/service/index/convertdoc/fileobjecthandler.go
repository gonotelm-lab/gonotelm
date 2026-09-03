package convertdoc

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourceerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	myparser "github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/convertdoc/parser"
	mytransformer "github.com/gonotelm-lab/gonotelm/internal/domain/source/service/index/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/eino/components/document"
	einoparser "github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*FileObjectHandler)(nil)

type FileObjectHandler struct {
	objectStorage repository.FileObjectGetter
	baseHandler   *baseHandler

	c HandlerConfig
}

func NewFileObjectHandler(
	hc HandlerConfig,
	objGetter repository.FileObjectGetter,
	imageInterpreter adapter.ImageInterpreter,
) *FileObjectHandler {
	return &FileObjectHandler{
		c:             hc,
		objectStorage: objGetter,
		baseHandler:   newBaseHandler("file-object-pipe", myparser.NewFileObjectHandler(imageInterpreter), hc),
	}
}

func (h *FileObjectHandler) Handle(
	ctx context.Context,
	src *entity.Source,
	opts ...HandleOption,
) (*HandleResult, error) {
	fs, err := src.GetFileContent()
	if err != nil {
		return nil, errors.Wrap(err, "get file content failed")
	}

	objBody, ok, err := h.loadObjectBody(ctx, fs.StoreKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &HandleResult{}, nil
	}

	parseOpts, transformOpts := fileConversionOptions(fs)
	docs, converted, err := h.baseHandler.doHandle(
		ctx,
		src,
		bytes.NewReader(objBody),
		append([]HandleOption{}, opts...),
		parseOpts,
		transformOpts...,
	)
	if err != nil {
		return nil, errors.Wrap(err, "handle file source failed")
	}

	return &HandleResult{
		Docs:              docs,
		ParsedContent:     converted,
		ParsedContentType: entity.MimeTypeMarkdown,
	}, nil
}

func fileConversionOptions(fs *entity.FileSourceContent) ([]einoparser.Option, []document.TransformerOption) {
	fileExt := filepath.Ext(fs.Filename)
	sourceMime := myparser.ResolveSourceMime(fs.Format, fileExt)
	parseOpts := []einoparser.Option{
		myparser.WithFileMime(fs.Format),
		myparser.WithFileExt(fileExt),
	}
	transformOpts := []document.TransformerOption{
		mytransformer.WithChunkSplitMethodByMime(sourceMime),
	}
	return parseOpts, transformOpts
}

func (h *FileObjectHandler) loadObjectBody(ctx context.Context, storeKey string) ([]byte, bool, error) {
	objBody, info, err := h.objectStorage.GetObject(ctx, storeKey)
	if err != nil {
		if errors.Is(err, repository.ErrObjectNotFound) {
			slog.ErrorContext(ctx, "file source object not found", "store_key", storeKey)
			return nil, false, nil
		}

		return nil, false, errors.Wrap(err, "get file source object failed")
	}

	if info.Size > h.c.MaxSourceFileSizeBytes {
		return nil, false, errors.Wrapf(
			sourceerr.ErrSourceContentTooLarge,
			"file source object size exceeds max size, size=%d",
			info.Size,
		)
	}

	return objBody, true, nil
}
