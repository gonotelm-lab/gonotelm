package convertdoc

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"

	convertdocparser "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/parser"
	convertdoctransformer "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/eino/components/document"
	einoparser "github.com/cloudwego/eino/components/document/parser"
)

var _ Handler = (*FileObjectHandler)(nil)

type FileObjectHandler struct {
	objectStorage storage.Storage
	impl          *commonHandler
}

func NewFileObjectHandler(c HandlerConfig, objectStorage storage.Storage) *FileObjectHandler {
	return &FileObjectHandler{
		objectStorage: objectStorage,
		impl:          newCommonHandler("file-object-pipe", &convertdocparser.FileObjectParser{}, c),
	}
}

func (e *FileObjectHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	fs := model.FileSourceContent{}
	if err := decodeSourceContent(s.Content, &fs, "unmarshal file source content failed"); err != nil {
		return nil, err
	}

	objBody, ok, err := e.loadObjectBody(ctx, fs.StoreKey)
	if err != nil {
		return nil, err
	}
	if !ok {
		return &HandleResult{}, nil
	}

	parseOpts, transformOpts := fileConversionOptions(fs)
	docs, err := e.impl.doHandle(
		ctx,
		s,
		bytes.NewReader(objBody),
		parseOpts,
		transformOpts...,
	)
	if err != nil {
		return nil, errors.Wrap(err, "handle file source failed")
	}

	return &HandleResult{Docs: docs}, nil
}

func fileConversionOptions(fs model.FileSourceContent) ([]einoparser.Option, []document.TransformerOption) {
	fileExt := filepath.Ext(fs.Filename)
	sourceMime := convertdocparser.ResolveSourceMime(fs.Format, fileExt)
	parseOpts := []einoparser.Option{
		convertdocparser.WithFileMime(fs.Format),
		convertdocparser.WithFileExt(fileExt),
	}
	transformOpts := []document.TransformerOption{
		convertdoctransformer.WithChunkSplitMethodByMime(sourceMime),
	}
	return parseOpts, transformOpts
}

func (e *FileObjectHandler) loadObjectBody(ctx context.Context, storeKey string) ([]byte, bool, error) {
	obj, err := e.objectStorage.GetObject(ctx, &storage.GetObjectRequest{
		Key: storeKey,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			slog.ErrorContext(ctx, "file source object not found", "store_key", storeKey)
			return nil, false, nil
		}
		return nil, false, errors.Wrap(err, "get file source object failed")
	}

	return obj.Body, true, nil
}
