package convertdoc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"

	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/token"

	"github.com/cloudwego/eino/components/document"
	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

const (
	parserMetaSourceObjKey        = "source_obj"
	parserMetaSourceIdKey         = "source_id"
	parserMetaSourceNotebookIdKey = "source_notebook_id"
	parserMetaSourceKindKey       = "source_kind"
)

type HandlerConfig struct {
	ChunkSize   int
	OverlapSize int
}

type HandleResult struct {
	Docs []*schema.Document

	ParsedContent     []byte
	ParsedContentType string
}

// 一系列hook
type HandlerHooks struct {
	// 解析文档前 返回error会中断后续处理
	BeforeParse func(ctx context.Context, source *model.Source, r io.Reader) error

	// 解析文档后 返回error会中断后续处理
	// 返回的docs会替换原来的docs 如果不处理 原样返回即可
	AfterParse func(
		ctx context.Context, source *model.Source, docs []*schema.Document) (
		[]*schema.Document, error,
	)

	// 执行transform前 返回error会中断后续处理
	BeforeTransform func(
		ctx context.Context, source *model.Source, docs []*schema.Document) error
}

type skipTransformIfFunc func(
	source *model.Source,
	parsedDocs []*schema.Document,
	parsedContent []byte,
) bool

type handleOptionImpl struct {
	// 跳过Transform阶段
	skipTransform   bool

	// 跳过Transform阶段;
	// 和skipTransform二选一 优先级skipTransform更高
	skipTransformIf skipTransformIfFunc
}

type HandleOption func(o *handleOptionImpl)

func WithHandleSkipTransform(b bool) HandleOption {
	return func(o *handleOptionImpl) {
		o.skipTransform = b
	}
}

func WithHandleSkipTransformIf(f skipTransformIfFunc) HandleOption {
	return func(o *handleOptionImpl) {
		o.skipTransformIf = f
	}
}

// Handler handles the source content before doing the actual embedding
// Actions include: parser, transformation, etc.
type Handler interface {
	Handle(ctx context.Context, s *model.Source, opts ...HandleOption) (*HandleResult, error)
}

// parsing + chunking
type baseHandler struct {
	name         string
	parser       einoparser.Parser // 最好统一parse成markdown格式
	transformers []document.Transformer

	// hooks
	hooks HandlerHooks
}

func newBaseHandler(name string, docParser einoparser.Parser, c HandlerConfig) *baseHandler {
	return &baseHandler{
		name:         name,
		parser:       docParser,
		transformers: defaultDocTransformer(c),
	}
}

func (h *baseHandler) doHandle(
	ctx context.Context,
	source *model.Source,
	r io.Reader,
	handleOpts []HandleOption,
	parseOpts []einoparser.Option,
	transformOpts ...document.TransformerOption,
) ([]*schema.Document, []byte, error) {
	handleOptImpl := &handleOptionImpl{}
	for _, opt := range handleOpts {
		opt(handleOptImpl)
	}

	if h.hooks.BeforeParse != nil {
		err := h.hooks.BeforeParse(ctx, source, r)
		if err != nil {
			return nil, nil, err
		}
	}

	// !note: make sure docs only contains one document
	docs, err := h.parse(ctx, r, parseOpts...)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "parse document failed, id=%s, pipeline=%s", source.Id, h.name)
	}
	if h.hooks.AfterParse != nil {
		newDocs, err := h.hooks.AfterParse(ctx, source, docs)
		if err != nil {
			return docs, nil, err
		}
		docs = newDocs
	}

	convertedDoc := docs[0].Content
	cvdBytes := pkgstring.AsBytes(convertedDoc)

	h.injectSourceMeta(source, docs)

	// 设置了只parse不transform直接返回即可
	if handleOptImpl.skipTransform {
		return docs, cvdBytes, nil
	}

	if handleOptImpl.skipTransformIf != nil {
		if handleOptImpl.skipTransformIf(source, docs, cvdBytes) {
			return docs, cvdBytes, nil
		}
	}

	if h.hooks.BeforeTransform != nil {
		err := h.hooks.BeforeTransform(ctx, source, docs)
		if err != nil {
			return docs, cvdBytes, err
		}
	}
	docs = h.transform(ctx, source, docs, transformOpts...)

	return docs, cvdBytes, nil
}

func (h *baseHandler) parse(
	ctx context.Context,
	r io.Reader,
	parseOpts ...einoparser.Option,
) ([]*schema.Document, error) {
	return h.parser.Parse(ctx, r, parseOpts...)
}

func (h *baseHandler) injectSourceMeta(
	source *model.Source,
	docs []*schema.Document,
) {
	customMetas := map[string]any{
		parserMetaSourceObjKey:        source,
		parserMetaSourceIdKey:         source.Id,
		parserMetaSourceNotebookIdKey: source.NotebookId,
		parserMetaSourceKindKey:       source.Kind,
	}
	for _, doc := range docs {
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}
		maps.Copy(doc.MetaData, customMetas)
	}
}

func (h *baseHandler) transform(
	ctx context.Context,
	source *model.Source,
	docs []*schema.Document,
	opts ...document.TransformerOption,
) []*schema.Document {
	for idx, transformer := range h.transformers {
		if transformer == nil {
			continue
		}

		newDocs, err := transformer.Transform(ctx, docs, opts...)
		if err != nil {
			slog.WarnContext(ctx,
				fmt.Sprintf("source handle pipeline %s transformer[%d] failed", h.name, idx),
				"source_id", source.Id, "err", err, "pipeline_name", h.name)
			continue
		}

		docs = newDocs
	}

	return docs
}

func defaultDocTransformer(c HandlerConfig) []document.Transformer {
	return []document.Transformer{
		transformer.NewChunkTransformer(c.ChunkSize, c.OverlapSize, token.Estimate),
	}
}

func decodeSourceContent(content []byte, decoder model.FromBytes, errMsg string) error {
	if err := decoder.From(content); err != nil {
		return errors.Wrap(errors.ErrSerde, errMsg)
	}

	return nil
}
