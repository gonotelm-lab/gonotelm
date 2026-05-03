package convertdoc

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

type LenCalulator func(string) int

type ChunkTransformer struct {
	html      document.Transformer
	markdown  document.Transformer
	recursive document.Transformer

	chunkSize   int
	overlapSize int
	lenFn       LenCalulator
}

func NewChunkTransformer(chunkSize, overlapSize int, lenFn LenCalulator) *ChunkTransformer {
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
	ret := make([]*schema.Document, 0, len(docs))

	for _, doc := range docs {
		var (
			tmpRets         []*schema.Document
			err             error
			secondCheckDocs = []*schema.Document{}
		)

		switch getChunkSplitMethod(doc) {
		case chunkHtmlSplitMethod:
			var tmpDocs []*schema.Document
			tmpDocs, err = t.html.Transform(ctx, []*schema.Document{doc}, opts...)
			secondCheckDocs = append(secondCheckDocs, tmpDocs...)
		case chunkMarkdownSplitMethod:
			var tmpDocs []*schema.Document
			tmpDocs, err = t.markdown.Transform(ctx, []*schema.Document{doc}, opts...)
			secondCheckDocs = append(secondCheckDocs, tmpDocs...)
		default:
			tmpRets, err = t.recursive.Transform(ctx, []*schema.Document{doc}, opts...)
		}
		if err != nil {
			return nil, errors.Wrap(err, "chunk transformer transform failed")
		}

		if len(secondCheckDocs) > 0 {
			for _, doc := range secondCheckDocs {
				if t.lenFn(doc.Content) > t.chunkSize {
					// do it again
					docAgain, err := t.recursive.Transform(ctx, []*schema.Document{doc}, opts...)
					if err != nil {
						return nil, errors.Wrap(err, "chunk transformer transform failed")
					}
					ret = append(ret, docAgain...)
				} else {
					ret = append(ret, doc)
				}
			}
		} else {
			ret = append(ret, tmpRets...)
		}
	}

	// filter empty chunks
	ret = filterEmptyDocs(ret)
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
