package transformer

import (
	"context"
	"strings"
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

const (
	ChunkMetaPositionStartKey     = "doc_pos_rune_start"
	ChunkMetaPositionEndKey       = "doc_pos_rune_end"
	ChunkMetaPositionByteStartKey = "doc_pos_byte_start"
	ChunkMetaPositionByteEndKey   = "doc_pos_byte_end"

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
	runeIndexByByteOffset := buildRuneIndexByByteOffset(sourceContent)

	var (
		prevChunkContent string
		prevStartByte    int
		prevEndByte      int
		hasPrevPos       bool
	)

	for _, doc := range docs {
		if doc == nil || doc.Content == "" {
			continue
		}

		startByte, ok := locateChunkStart(
			sourceContent,
			doc.Content,
			prevChunkContent,
			prevStartByte,
			prevEndByte,
			hasPrevPos,
		)
		if !ok {
			continue
		}

		endByte := startByte + len(doc.Content)
		startRune := byteOffsetToRuneOffset(runeIndexByByteOffset, startByte)
		endRune := byteOffsetToRuneOffset(runeIndexByByteOffset, endByte)
		setChunkPositionMeta(doc, startByte, endByte, startRune, endRune)

		prevChunkContent = doc.Content
		prevStartByte = startByte
		prevEndByte = endByte
		hasPrevPos = true
	}
}

func locateChunkStart(
	source string,
	chunk string,
	prevChunk string,
	prevStart int,
	prevEnd int,
	hasPrevPos bool,
) (int, bool) {
	if chunk == "" {
		return 0, false
	}

	if !hasPrevPos {
		idx := strings.Index(source, chunk)
		return idx, idx >= 0
	}

	if expected, ok := locateChunkStartByOverlap(source, chunk, prevChunk, prevEnd); ok {
		return expected, true
	}

	if idx, ok := locateChunkStartByForwardScan(source, chunk, prevStart, prevEnd); ok {
		return idx, true
	}

	if prevEnd >= 0 && prevEnd < len(source) {
		idx := strings.Index(source[prevEnd:], chunk)
		if idx >= 0 {
			return prevEnd + idx, true
		}
	}

	idx := strings.Index(source, chunk)
	return idx, idx >= 0
}

func locateChunkStartByOverlap(source, chunk, prevChunk string, prevEnd int) (int, bool) {
	if prevChunk == "" {
		return 0, false
	}

	overlap := longestSuffixPrefixOverlap(prevChunk, chunk)
	if overlap == 0 || overlap == len(chunk) {
		return 0, false
	}

	start := prevEnd - overlap
	end := start + len(chunk)
	if start < 0 || end > len(source) {
		return 0, false
	}
	if end <= prevEnd {
		return 0, false
	}

	if source[start:end] == chunk {
		return start, true
	}

	return 0, false
}

func locateChunkStartByForwardScan(source, chunk string, scanStart, prevEnd int) (int, bool) {
	if scanStart < 0 {
		scanStart = 0
	}
	if scanStart >= len(source) {
		return 0, false
	}

	for scanStart < len(source) {
		idx := strings.Index(source[scanStart:], chunk)
		if idx < 0 {
			return 0, false
		}

		candidateStart := scanStart + idx
		candidateEnd := candidateStart + len(chunk)
		if candidateEnd > len(source) {
			return 0, false
		}

		// 对于有重叠分块的场景，允许 candidateStart < prevEnd；
		// 但如果整个 chunk 完全落在前一个 chunk 内，则通常是重复文本导致的错误命中。
		if candidateStart < prevEnd && candidateEnd <= prevEnd {
			scanStart = candidateStart + 1
			continue
		}

		return candidateStart, true
	}

	return 0, false
}

func longestSuffixPrefixOverlap(left, right string) int {
	max := len(left)
	if len(right) < max {
		max = len(right)
	}

	for size := max; size > 0; size-- {
		if left[len(left)-size:] == right[:size] {
			return size
		}
	}

	return 0
}

func buildRuneIndexByByteOffset(source string) []int {
	index := make([]int, len(source)+1)

	runeIdx := 0
	for i := 0; i < len(source); {
		_, size := utf8.DecodeRuneInString(source[i:])
		for j := 0; j < size && i+j < len(source); j++ {
			index[i+j] = runeIdx
		}
		i += size
		index[i] = runeIdx + 1
		runeIdx++
	}

	return index
}

func byteOffsetToRuneOffset(runeIndexByByteOffset []int, byteOffset int) int {
	if len(runeIndexByByteOffset) == 0 {
		return 0
	}
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(runeIndexByByteOffset) {
		return runeIndexByByteOffset[len(runeIndexByByteOffset)-1]
	}
	return runeIndexByByteOffset[byteOffset]
}

func setChunkPositionMeta(doc *schema.Document, startByte, endByte, startRune, endRune int) {
	if doc.MetaData == nil {
		doc.MetaData = make(map[string]any, 4)
	}

	doc.MetaData[ChunkMetaPositionStartKey] = startRune
	doc.MetaData[ChunkMetaPositionEndKey] = endRune
	doc.MetaData[ChunkMetaPositionByteStartKey] = startByte
	doc.MetaData[ChunkMetaPositionByteEndKey] = endByte
}
