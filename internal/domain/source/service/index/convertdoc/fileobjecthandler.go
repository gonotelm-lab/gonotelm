package convertdoc

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"

	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
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

func NewFileObjectHandler(c HandlerConfig, objGetter repository.FileObjectGetter) *FileObjectHandler {
	return &FileObjectHandler{
		c:             c,
		objectStorage: objGetter,
		baseHandler:   newBaseHandler("file-object-pipe", &myparser.FileObjectParser{}, c),
	}
}

func (e *FileObjectHandler) Handle(
	ctx context.Context,
	src *entity.Source,
	opts ...HandleOption,
) (*HandleResult, error) {
	fs, err := src.GetFileContent()
	if err != nil {
		return nil, errors.Wrap(err, "get file content failed")
	}

	objBody, ok, err := e.loadObjectBody(ctx, fs.StoreKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &HandleResult{}, nil
	}

	parseOpts, transformOpts := fileConversionOptions(fs)
	docs, converted, err := e.baseHandler.doHandle(
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

func (e *FileObjectHandler) loadObjectBody(ctx context.Context, storeKey string) ([]byte, bool, error) {
	objBody, size, err := e.objectStorage.GetObject(ctx, storeKey)
	if err != nil {
		if errors.Is(err, repository.ErrObjectNotFound) {
			slog.ErrorContext(ctx, "file source object not found", "store_key", storeKey)
			return nil, false, nil
		}

		return nil, false, errors.Wrap(err, "get file source object failed")
	}

	if size > e.c.MaxSourceFileSizeBytes {
		return nil, false, errors.ErrParams.Msgf("file source object size exceeds max size, size=%d", size)
	}

	return objBody, true, nil
}
