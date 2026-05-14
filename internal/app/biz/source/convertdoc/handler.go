package convertdoc

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"

	convertdoctransformer "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/transformer"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/token"

	"github.com/cloudwego/eino/components/document"
	einoparser "github.com/cloudwego/eino/components/document/parser"
	"github.com/cloudwego/eino/schema"
)

const (
	parserMetaSourceObjKey        = "_gonotelm_source_obj"
	parserMetaSourceIdKey         = "_gonotelm_source_id"
	parserMetaSourceNotebookIdKey = "_gonotelm_source_notebook_id"
	parserMetaSourceKindKey       = "_gonotelm_source_kind"
)

type HandlerConfig struct {
	ChunkSize   int
	OverlapSize int
}

type HandleResult struct {
	Docs []*schema.Document
}

// Handler handles the source content before doing the actual embedding
// Actions include: parser, transformation, etc.
type Handler interface {
	Handle(ctx context.Context, s *model.Source) (*HandleResult, error)
}

// parsing + chunking
type commonHandler struct {
	name         string
	parser       einoparser.Parser // 最好统一parse成markdown格式
	transformers []document.Transformer
}

func newCommonHandler(name string, docParser einoparser.Parser, c HandlerConfig) *commonHandler {
	return &commonHandler{
		name:         name,
		parser:       docParser,
		transformers: defaultDocTransformer(c),
	}
}

func (h *commonHandler) doHandle(
	ctx context.Context,
	source *model.Source,
	r io.Reader,
	parseOpts []einoparser.Option,
	transformOpts ...document.TransformerOption,
) ([]*schema.Document, error) {
	docs, err := h.parse(ctx, r, parseOpts...)
	if err != nil {
		return nil, errors.Wrapf(err, "parse document failed, id=%s, pipeline=%s", source.Id, h.name)
	}

	h.injectSourceMeta(source, docs)
	docs = h.transform(ctx, source, docs, transformOpts...)
	return docs, nil
}

func (h *commonHandler) parse(
	ctx context.Context,
	r io.Reader,
	parseOpts ...einoparser.Option,
) ([]*schema.Document, error) {
	return h.parser.Parse(ctx, r, parseOpts...)
}

func (h *commonHandler) injectSourceMeta(
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

func (h *commonHandler) transform(
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
		convertdoctransformer.NewChunkTransformer(c.ChunkSize, c.OverlapSize, token.EstimateToken),
	}
}

func decodeSourceContent(content []byte, decoder model.FromBytes, errMsg string) error {
	if err := decoder.From(content); err != nil {
		return errors.Wrap(errors.ErrSerde, errMsg)
	}

	return nil
}
