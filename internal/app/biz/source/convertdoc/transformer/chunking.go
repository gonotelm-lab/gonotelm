package transformer

import (
	"context"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einohtml "github.com/cloudwego/eino-ext/components/document/transformer/splitter/html"
	eniomarkdown "github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	einorecursive "github.com/cloudwego/eino-ext/components/document/transformer/splitter/recursive"
)

func splitDocIdGenerator(ctx context.Context, originalID string, splitIndex int) string {
	return uuid.NewV4().String()
}

type LenCalculator func(string) int

// 保留旧命名兼容历史调用。
type LenCalulator = LenCalculator

type ChunkTransformer struct {
	html      document.Transformer
	markdown  document.Transformer
	recursive document.Transformer

	chunkSize   int
	overlapSize int
	lenFn       LenCalculator
}

func NewChunkTransformer(chunkSize, overlapSize int, lenFn LenCalculator) *ChunkTransformer {
	ctx := context.Background()
	ht, _ := einohtml.NewHeaderSplitter(ctx, &einohtml.HeaderConfig{
		IDGenerator: func(ctx context.Context, originalID string, splitIndex int) string {
			return splitDocIdGenerator(ctx, originalID, splitIndex)
		},
		Headers: map[string]string{
			"h1": "_gonotelm_doc_h1",
			"h2": "_gonotelm_doc_h2",
			"h3": "_gonotelm_doc_h3",
			"h4": "_gonotelm_doc_h4",
			"h5": "_gonotelm_doc_h5",
			"h6": "_gonotelm_doc_h6",
		},
	})

	mt, _ := eniomarkdown.NewHeaderSplitter(ctx, &eniomarkdown.HeaderConfig{
		IDGenerator: splitDocIdGenerator,
		Headers: map[string]string{
			"#":      "_gonotelm_doc_h1",
			"##":     "_gonotelm_doc_h2",
			"###":    "_gonotelm_doc_h3",
			"####":   "_gonotelm_doc_h4",
			"#####":  "_gonotelm_doc_h5",
			"######": "_gonotelm_doc_h6",
		},
	})

	if chunkSize <= 0 {
		chunkSize = 500
	}
	if overlapSize < 0 {
		overlapSize = 75
	}
	if lenFn == nil {
		lenFn = func(s string) int { return utf8.RuneCountInString(s) }
	}

	rt, _ := einorecursive.NewSplitter(ctx, &einorecursive.Config{
		ChunkSize:   chunkSize,
		OverlapSize: overlapSize,
		IDGenerator: splitDocIdGenerator,
		LenFunc:     lenFn,
		Separators:  []string{"\n\n", "\n", ". ", ", ", " ", "", "?", "!", "，", "。", "？", "！"},
	})

	return &ChunkTransformer{
		html:      ht,
		markdown:  mt,
		recursive: rt,

		chunkSize:   chunkSize,
		overlapSize: overlapSize,
		lenFn:       lenFn,
	}
}

var _ document.Transformer = (*ChunkTransformer)(nil)

func (t *ChunkTransformer) Transform(
	ctx context.Context,
	docs []*schema.Document,
	opts ...document.TransformerOption,
) ([]*schema.Document, error) {
	splitMethod := GetChunkSplitMethodOption(opts...)
	ret := make([]*schema.Document, 0, len(docs))

	for _, doc := range docs {
		docChunks, err := t.splitDoc(ctx, doc, splitMethod, opts...)
		if err != nil {
			return nil, errors.Wrap(err, "chunk transformer transform failed")
		}
		ret = append(ret, docChunks...)
	}

	// filter empty chunks
	return filterEmptyDocs(ret), nil
}

func (t *ChunkTransformer) splitDoc(
	ctx context.Context,
	doc *schema.Document,
	splitMethod string,
	opts ...document.TransformerOption,
) ([]*schema.Document, error) {
	splitDocs, needSecondCheck, err := t.splitByMethod(ctx, doc, splitMethod, opts...)
	if err != nil {
		return nil, err
	}

	if !needSecondCheck {
		return splitDocs, nil
	}

	return t.applyRecursiveFallback(ctx, splitDocs, opts...)
}

func (t *ChunkTransformer) splitByMethod(
	ctx context.Context,
	doc *schema.Document,
	splitMethod string,
	opts ...document.TransformerOption,
) (docs []*schema.Document, needSecondCheck bool, err error) {
	switch normalizeChunkSplitMethod(splitMethod) {
	case ChunkHtmlSplitMethod:
		docs, err = t.html.Transform(ctx, []*schema.Document{doc}, opts...)
		return docs, true, err
	case ChunkMarkdownSplitMethod:
		docs, err = t.markdown.Transform(ctx, []*schema.Document{doc}, opts...)
		return docs, true, err
	default:
		docs, err = t.recursive.Transform(ctx, []*schema.Document{doc}, opts...)
		return docs, false, err
	}
}

func (t *ChunkTransformer) applyRecursiveFallback(
	ctx context.Context,
	docs []*schema.Document,
	opts ...document.TransformerOption,
) ([]*schema.Document, error) {
	ret := make([]*schema.Document, 0, len(docs))
	for _, doc := range docs {
		if t.lenFn(doc.Content) > t.chunkSize {
			docAgain, err := t.recursive.Transform(ctx, []*schema.Document{doc}, opts...)
			if err != nil {
				return nil, err
			}
			ret = append(ret, docAgain...)
			continue
		}
		ret = append(ret, doc)
	}
	return ret, nil
}

func filterEmptyDocs(docs []*schema.Document) []*schema.Document {
	ret := make([]*schema.Document, 0, len(docs))

	for _, doc := range docs {
		if doc.Content != "" {
			ret = append(ret, doc)
		}
	}

	return ret
}
