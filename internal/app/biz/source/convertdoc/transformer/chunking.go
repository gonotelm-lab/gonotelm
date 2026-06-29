package transformer

import (
	"context"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/gonotelm-lab/gonotelm/internal/app/biz/source/util"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/chunker/recursive"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einohtml "github.com/cloudwego/eino-ext/components/document/transformer/splitter/html"
	eniomarkdown "github.com/cloudwego/eino-ext/components/document/transformer/splitter/markdown"
	markdownmeta "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/chunker/markdown"
)

func splitDocIdGenerator(ctx context.Context, originalID string, splitIndex int) string {
	return uuid.NewV4().String()
}

type LenCalculator func(string) int

const (
	ChunkMetaPosStartKey     = model.ChunkMetaPosStartKey
	ChunkMetaPosEndKey       = model.ChunkMetaPosEndKey
	ChunkMetaPosByteStartKey = model.ChunkMetaPosByteStartKey
	ChunkMetaPosByteEndKey   = model.ChunkMetaPosByteEndKey

	ChunkMetaDocH1Key = "doc_h1"
	ChunkMetaDocH2Key = "doc_h2"
	ChunkMetaDocH3Key = "doc_h3"
	ChunkMetaDocH4Key = "doc_h4"
	ChunkMetaDocH5Key = "doc_h5"
	ChunkMetaDocH6Key = "doc_h6"
)

const (
	defaultChunkSize   = 500
	defaultOverlapSize = 75
)

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
			"h1": ChunkMetaDocH1Key,
			"h2": ChunkMetaDocH2Key,
			"h3": ChunkMetaDocH3Key,
			"h4": ChunkMetaDocH4Key,
			"h5": ChunkMetaDocH5Key,
			"h6": ChunkMetaDocH6Key,
		},
	})

	mt, _ := eniomarkdown.NewHeaderSplitter(ctx, &eniomarkdown.HeaderConfig{
		IDGenerator: splitDocIdGenerator,
		Headers: map[string]string{
			"#":      ChunkMetaDocH1Key,
			"##":     ChunkMetaDocH2Key,
			"###":    ChunkMetaDocH3Key,
			"####":   ChunkMetaDocH4Key,
			"#####":  ChunkMetaDocH5Key,
			"######": ChunkMetaDocH6Key,
		},
	})

	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}
	if overlapSize < 0 {
		overlapSize = defaultOverlapSize
	}
	if lenFn == nil {
		lenFn = func(s string) int { return utf8.RuneCountInString(s) }
	}

	rt, _ := recursive.NewSplitter(ctx,
		&recursive.Config{
			ChunkSize:   chunkSize,
			OverlapSize: overlapSize,
			IDGenerator: splitDocIdGenerator,
			LenFunc:     lenFn,
			KeepType:    recursive.KeepTypeEnd,
			Separators:  []string{"\n\n", "\n", ".", " ", "", "?", "!", "。", "？", "！"},
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

		annotateChunkPositions(doc.Content, docChunks) // try to annotate chunk positions
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
	splitDocs, needSecondCheck, err := t.splitByMethod(
		ctx, doc, splitMethod, opts...,
	)
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

func annotateChunkPositions(sourceContent string, docs []*schema.Document) {
	if sourceContent == "" || len(docs) == 0 {
		return
	}

	needCompute := make([]*schema.Document, 0, len(docs))
	needContents := make([]string, 0, len(docs))
	for _, doc := range docs {
		if doc == nil || doc.Content == "" {
			continue
		}
		if tryUseEmbeddedPositionMeta(sourceContent, doc) {
			continue
		}
		needCompute = append(needCompute, doc)
		needContents = append(needContents, doc.Content)
	}

	if len(needCompute) == 0 {
		return
	}

	runeIndexByByteOffset := util.BuildRuneIndexByByteOffset(sourceContent)
	chunkSpans := util.BuildChunkByteSpans(sourceContent, needContents)
	for idx, doc := range needCompute {
		span := chunkSpans[idx]
		if span.StartByte < 0 || span.EndByte <= span.StartByte {
			continue
		}
		startByte := span.StartByte
		endByte := span.EndByte
		startRune := util.ByteOffsetToRuneOffset(runeIndexByByteOffset, startByte)
		endRune := util.ByteOffsetToRuneOffset(runeIndexByByteOffset, endByte)
		setChunkPositionMeta(doc, startByte, endByte, startRune, endRune)
	}
}

func tryUseEmbeddedPositionMeta(sourceContent string, doc *schema.Document) bool {
	mdByteStart, ok1 := getMetaInt(doc.MetaData, markdownmeta.MetaChunkByteStartKey)
	mdByteEnd, ok2 := getMetaInt(doc.MetaData, markdownmeta.MetaChunkByteEndKey)
	mdRuneStart, ok3 := getMetaInt(doc.MetaData, markdownmeta.MetaChunkRuneStartKey)
	mdRuneEnd, ok4 := getMetaInt(doc.MetaData, markdownmeta.MetaChunkRuneEndKey)
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return false
	}
	if mdByteStart < 0 || mdByteEnd <= mdByteStart || mdByteEnd > len(sourceContent) {
		return false
	}
	if sourceContent[mdByteStart:mdByteEnd] != doc.Content {
		return false
	}
	setChunkPositionMeta(doc, mdByteStart, mdByteEnd, mdRuneStart, mdRuneEnd)
	return true
}

func getMetaInt(meta map[string]any, key string) (int, bool) {
	raw, ok := meta[key]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func setChunkPositionMeta(doc *schema.Document, startByte, endByte, startRune, endRune int) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any, 4)
	}

	doc.MetaData[ChunkMetaPosStartKey] = startRune
	doc.MetaData[ChunkMetaPosEndKey] = endRune
	doc.MetaData[ChunkMetaPosByteStartKey] = startByte
	doc.MetaData[ChunkMetaPosByteEndKey] = endByte
}
