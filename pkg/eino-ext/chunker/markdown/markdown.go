package markdown

import (
	"context"
	"fmt"
	"maps"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type IDGenerator func(ctx context.Context, originalID string, splitIndex int) string

func defaultIDGenerator(_ context.Context, originalID string, _ int) string {
	return originalID
}

type Config struct {
	ChunkSize   int
	Overlap     int
	LenFunc     func(string) int
	IDGenerator IDGenerator
}

func NewSplitter(_ context.Context, config *Config) (document.Transformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	if config.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be greater than zero")
	}

	lenFn := config.LenFunc
	if lenFn == nil {
		lenFn = func(s string) int { return len(s) }
	}

	idGen := config.IDGenerator
	if idGen == nil {
		idGen = defaultIDGenerator
	}

	overlap := config.Overlap
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= config.ChunkSize {
		return nil, fmt.Errorf("overlap must be less than chunk size")
	}

	return &markdownChunker{
		chunkSize:   config.ChunkSize,
		overlap:     overlap,
		lenFn:       lenFn,
		idGenerator: idGen,
	}, nil
}

type markdownChunker struct {
	chunkSize   int
	overlap     int
	lenFn       func(string) int
	idGenerator IDGenerator
}

var _ document.Transformer = (*markdownChunker)(nil)

func (c *markdownChunker) Transform(
	ctx context.Context,
	docs []*schema.Document,
	_ ...document.TransformerOption,
) ([]*schema.Document, error) {
	ret := make([]*schema.Document, 0)

	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if doc == nil {
			continue
		}

		drafts := splitSource(doc.Content, c.chunkSize, c.overlap, c.lenFn)
		for i, draft := range drafts {
			if draft.content == "" {
				continue
			}

			meta := deepCopyMap(doc.MetaData)
			for k, v := range draft.meta {
				meta[k] = v
			}

			ret = append(ret, &schema.Document{
				ID:       c.idGenerator(ctx, doc.ID, i),
				Content:  draft.content,
				MetaData: meta,
			})
		}
	}

	return ret, nil
}

func (c *markdownChunker) GetType() string {
	return "MarkdownChunker"
}

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}

	ret := make(map[string]any, len(m))
	maps.Copy(ret, m)
	return ret
}
