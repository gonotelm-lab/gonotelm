package convertdoc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/token"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

type HandlerConfig struct {
	ChunkSize   int
	OverlapSize int
}

// parsing + chunking
type handlerPipeline struct {
	name         string
	parser       parser.Parser // 最好统一parse成markdown格式
	transformers []document.Transformer
}

func (p *handlerPipeline) pipe(
	ctx context.Context,
	source *model.Source,
	r io.Reader,
	parseOpts ...parser.Option,
) ([]*schema.Document, error) {
	var docs []*schema.Document
	var err error
	docs, err = p.parser.Parse(ctx, r, append(parseOpts, withParseSource(source))...) // 此处统一注入source
	if err != nil {
		return nil, errors.Wrapf(err, "parse document failed, id=%s, pipeline=%s", source.Id, p.name)
	}

	customMetas := make(map[string]any)
	customMetas[parserMetaSourceObjKey] = source
	customMetas[parserMetaSourceIdKey] = source.Id
	customMetas[parserMetaSourceNotebookIdKey] = source.NotebookId
	customMetas[parserMetaSourceKindKey] = source.Kind

	// 对每个doc注入source
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}
		maps.Copy(doc.MetaData, customMetas)
	}

	for idx, transformer := range p.transformers {
		if transformer != nil {
			docs, err = transformer.Transform(ctx, docs)
			if err != nil {
				slog.WarnContext(ctx,
					fmt.Sprintf("source handle pipeline %s transformer[%d] failed", p.name, idx),
					"source_id", source.Id, "err", err, "pipeline_name", p.name)
				continue
			}
		}
	}

	return docs, nil
}

func defaultDocTransformer(c HandlerConfig) []document.Transformer {
	return []document.Transformer{
		NewChunkTransformer(c.ChunkSize, c.OverlapSize, token.EstimateToken),
	}
}

type HandleResult struct {
	Docs []*schema.Document
}

// Handler handles the source content before doing the actual embedding
// Actions include: parser, transformation, etc.
type Handler interface {
	Handle(ctx context.Context, s *model.Source) (*HandleResult, error)
}

var (
	_ Handler = (*TextHandler)(nil)
	_ Handler = (*UrlHandler)(nil)
	_ Handler = (*FileObjectHandler)(nil)
)

type TextHandler struct {
	pipe *handlerPipeline
}

func NewTextHandler(c HandlerConfig) *TextHandler {
	parser := parser.TextParser{}

	return &TextHandler{
		pipe: &handlerPipeline{
			name:         "text-pipe",
			parser:       parser,
			transformers: defaultDocTransformer(c),
		},
	}
}

func (e *TextHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	ts := model.TextSourceContent{}
	err := ts.From(s.Content)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, "unmarshal text source content failed")
	}

	docs, err := e.pipe.pipe(ctx, s, strings.NewReader(ts.Text))
	if err != nil {
		return nil, errors.Wrap(err, "handle text source failed")
	}

	return &HandleResult{Docs: docs}, nil
}

type UrlHandler struct {
	pipe *handlerPipeline
}

func NewUrlHandler(c HandlerConfig) *UrlHandler {
	parser := parser.TextParser{}
	// TODO
	return &UrlHandler{
		pipe: &handlerPipeline{
			name:         "url-pipe",
			parser:       parser,
			transformers: defaultDocTransformer(c),
		},
	}
}

func (e *UrlHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	us := model.UrlSourceContent{}
	err := us.From(s.Content)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, "unmarshal url source content failed")
	}

	return nil, errors.New("not implemented")
}

type FileObjectHandler struct {
	objectStorage storage.Storage
	pipe          *handlerPipeline
}

func NewFileObjectHandler(c HandlerConfig, objectStorage storage.Storage) *FileObjectHandler {
	return &FileObjectHandler{
		objectStorage: objectStorage,
		pipe: &handlerPipeline{
			name:         "file-object-pipe",
			parser:       &fileObjectParser{},
			transformers: defaultDocTransformer(c),
		},
	}
}

func (e *FileObjectHandler) Handle(ctx context.Context, s *model.Source) (*HandleResult, error) {
	fs := model.FileSourceContent{}
	err := fs.From(s.Content)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, "unmarshal file source content failed")
	}

	// load object
	obj, err := e.objectStorage.GetObject(ctx, &storage.GetObjectRequest{
		Key: fs.StoreKey,
	})
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotFound) {
			slog.ErrorContext(ctx, "file source object not found", "store_key", fs.StoreKey)
			return &HandleResult{}, nil
		}

		return nil, errors.Wrap(err, "get file source object failed")
	}

	reader := bytes.NewReader(obj.Body)
	docs, err := e.pipe.pipe(ctx, s, reader,
		withParseFileMime(fs.Format),
		withParseFileExt(filepath.Ext(fs.Filename)),
	)
	if err != nil {
		return nil, errors.Wrap(err, "handle file source failed")
	}

	return &HandleResult{Docs: docs}, nil
}
