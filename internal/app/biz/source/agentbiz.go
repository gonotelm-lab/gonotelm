package source

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"unicode/utf8"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/log"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/allegro/bigcache/v3"
)

type AgentBiz struct {
	impl        *Biz
	sourceCache *bigcache.BigCache
}

type AgentBizConfig struct {
	// 缓存的过期时间
	SourceCacheEviction time.Duration

	// 缓存的最大大小，单位MB
	SourceCacheMaxMB int
}

func NewAgentBiz(ctx context.Context, impl *Biz, c AgentBizConfig) (*AgentBiz, error) {
	if c.SourceCacheEviction <= 0 {
		c.SourceCacheEviction = time.Minute * 15
	}
	if c.SourceCacheMaxMB <= 0 {
		c.SourceCacheMaxMB = 512 // 512MB
	}

	config := bigcache.DefaultConfig(c.SourceCacheEviction)
	config.HardMaxCacheSize = c.SourceCacheMaxMB
	config.Logger = &log.BigcacheLogger{}
	config.Verbose = true

	sourceCache, err := bigcache.New(ctx, config)
	if err != nil {
		return nil, errors.Wrap(err, "new source cache failed")
	}

	return &AgentBiz{
		impl:        impl,
		sourceCache: sourceCache,
	}, nil
}

type lineRange struct {
	Start int `json:"s"`
	End   int `json:"e"`
}

type cachedSource struct {
	Content    []byte      `json:"c"`
	LineRanges []lineRange `json:"lr"`
	Abstract   string      `json:"a"`
}

// 获取来源全部内容
func (b *AgentBiz) GetSourceContent(ctx context.Context, id uuid.UUID) ([]byte, error) {
	sourceId := id.String()
	cacheKey := sourceId

	encodedPayload, err := b.sourceCache.Get(cacheKey)
	if err == nil {
		cached, decodeErr := b.decodeCachedSource(encodedPayload)
		if decodeErr == nil {
			return cached.Content, nil
		}
	}

	rawContent, _, _, err := b.fetchAndSetCache(ctx, id)
	if err != nil {
		return nil, err
	}

	return rawContent, nil
}

type AgentStatSourceResult struct {
	Bytes    int    `json:"b"` // content len in bytes
	Runes    int    `json:"r"` // content len in runes
	Lines    int    `json:"l"` // content line count
	Abstract string `json:"a"` // summary of the source content
}

func (b *AgentBiz) StatSource(ctx context.Context, id uuid.UUID) (*AgentStatSourceResult, error) {
	encodedPayload, err := b.sourceCache.Get(id.String())
	if err == nil {
		cached, decodeErr := b.decodeCachedSource(encodedPayload)
		if decodeErr == nil {
			return &AgentStatSourceResult{
				Bytes:    len(cached.Content),
				Runes:    utf8.RuneCount(cached.Content),
				Lines:    len(cached.LineRanges),
				Abstract: cached.Abstract,
			}, nil
		}
	}

	rawContent, lineRanges, abstract, err := b.fetchAndSetCache(ctx, id)
	if err != nil {
		return nil, err
	}

	return &AgentStatSourceResult{
		Bytes:    len(rawContent),
		Runes:    utf8.RuneCount(rawContent),
		Lines:    len(lineRanges),
		Abstract: abstract,
	}, nil
}

type AgentReadSourceQuery struct {
	SourceId uuid.UUID
	Offset   int // 起始行 1-indexed
	Limit    int // 读取的行数
}

type AgentReadSourceResult struct {
	TotalLines int
	Lines      []AgentReadSourceResultLine
}

type AgentReadSourceResultLine struct {
	LineNo int // 行号 1-indexed
	Line   []byte
}

// 为agent提供来源的内容
func (b *AgentBiz) ReadSource(
	ctx context.Context,
	query *AgentReadSourceQuery,
) (*AgentReadSourceResult, error) {
	sourceID := query.SourceId.String()
	cacheKey := sourceID

	encodedPayload, err := b.sourceCache.Get(cacheKey)
	if err == nil {
		cached, decodeErr := b.decodeCachedSource(encodedPayload)
		if decodeErr == nil {
			return b.selectContent(cached.Content, cached.LineRanges, query.Offset, query.Limit)
		}

		slog.WarnContext(ctx, "decode source cache failed, fallback to refetch",
			slog.Any("err", decodeErr),
			slog.String("source_id", sourceID),
		)
	}

	// cache miss or payload invalid, fetch from database and set local cache
	rawContent, lineRanges, _, err := b.fetchAndSetCache(ctx, query.SourceId)
	if err != nil {
		return nil, err
	}

	return b.selectContent(rawContent, lineRanges, query.Offset, query.Limit)
}

func (b *AgentBiz) fetchAndSetCache(ctx context.Context, sourceId uuid.UUID) ([]byte, []lineRange, string, error) {
	rawContent, abstract, err := b.fetchSourceContent(ctx, sourceId)
	if err != nil {
		return nil, nil, "", err
	}

	lineRanges := b.buildLineRanges(rawContent)
	newPayload, err := b.encodeCachedSource(rawContent, lineRanges, abstract)
	if err != nil {
		slog.ErrorContext(ctx, "encode source cache payload failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
		)

		return rawContent, lineRanges, abstract, err
	}

	if err := b.sourceCache.Set(sourceId.String(), newPayload); err != nil {
		slog.ErrorContext(ctx, "set source cache failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
		)
	}

	return rawContent, lineRanges, abstract, nil
}

func (b *AgentBiz) encodeCachedSource(
	content []byte,
	lineRanges []lineRange,
	abstract string,
) ([]byte, error) {
	return sonic.Marshal(cachedSource{
		Content:    content,
		LineRanges: lineRanges,
		Abstract:   abstract,
	})
}

func (b *AgentBiz) decodeCachedSource(payload []byte) (*cachedSource, error) {
	var c cachedSource
	if err := sonic.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("unmarshal cache payload failed: %w", err)
	}

	return &c, nil
}

func (b *AgentBiz) selectContent(
	rawContent []byte,
	lineRanges []lineRange,
	offset int,
	limit int,
) (*AgentReadSourceResult, error) {
	totalLines := len(lineRanges)
	start := 0
	if offset != 0 {
		start = offset - 1
	}
	start = max(start, 0)
	if start >= totalLines {
		return nil, fmt.Errorf(
			"source content has only %d lines, but offset %d is requested.",
			totalLines, offset,
		)
	}

	end := totalLines
	if limit != 0 {
		end = start + limit
	}
	end = min(end, totalLines)

	result := &AgentReadSourceResult{
		TotalLines: totalLines,
		Lines:      make([]AgentReadSourceResultLine, 0, end-start),
	}
	contentLen := len(rawContent)
	for i := start; i < end; i++ {
		lineRange := lineRanges[i]
		if lineRange.Start < 0 || lineRange.End < lineRange.Start || lineRange.End > contentLen {
			return nil, fmt.Errorf("invalid line range at line %d: start=%d end=%d content=%d",
				i+1, lineRange.Start, lineRange.End, contentLen)
		}
		result.Lines = append(result.Lines, AgentReadSourceResultLine{
			LineNo: i + 1,
			Line:   rawContent[lineRange.Start:lineRange.End],
		})
	}

	return result, nil
}

func (b *AgentBiz) fetchSourceContent(
	ctx context.Context,
	sourceId uuid.UUID,
) ([]byte, string, error) {
	src, err := b.impl.GetDecodedSource(ctx, sourceId)
	if err != nil {
		if errors.Is(err, ErrSourceNotFound) {
			return nil, "", fmt.Errorf("source not found, id=%s", sourceId)
		}

		slog.ErrorContext(ctx, "get decoded source failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
		)

		return nil, "", fmt.Errorf("get decoded source failed, id=%s, err=%w", sourceId, err)
	}

	if src.ParsedContentKey == "" {
		return nil, "", fmt.Errorf("source does not contain any valid parsed content key")
	}

	content, err := b.impl.objectStorage.GetObject(ctx,
		&storage.GetObjectRequest{
			Key: src.ParsedContentKey,
		},
	)
	if err != nil {
		return nil, "", fmt.Errorf("get parsed content failed, id=%s, err=%w", sourceId, err)
	}

	return content.Body, src.Abstract, nil
}

func (b *AgentBiz) buildLineRanges(rawContent []byte) []lineRange {
	lineRanges := make([]lineRange, 0, len(rawContent)/48+1)
	lineStart := 0

	for i, b := range rawContent {
		if b != '\n' {
			continue
		}

		lineEnd := i
		if lineEnd > lineStart && rawContent[lineEnd-1] == '\r' {
			lineEnd--
		}

		lineRanges = append(lineRanges, lineRange{
			Start: lineStart,
			End:   lineEnd,
		})
		lineStart = i + 1
	}

	if lineStart < len(rawContent) {
		lineRanges = append(lineRanges, lineRange{
			Start: lineStart,
			End:   len(rawContent),
		})
	}

	return lineRanges
}

type AgentSearchSourceQuery struct {
	NotebookId uuid.UUID
	SourceIds  []uuid.UUID
	Target     string
	Count      int
}

type AgentSearchSourceResult struct {
	Chunks []*model.SourceDoc
}

func (b *AgentBiz) SearchSource(
	ctx context.Context,
	query *AgentSearchSourceQuery,
) (*AgentSearchSourceResult, error) {
	sources, err := b.impl.BatchGetSources(ctx, query.NotebookId, query.SourceIds)
	if err != nil {
		slog.ErrorContext(ctx, "batch get sources failed",
			slog.Any("err", err),
			slog.Any("source_ids", query.SourceIds),
		)

		return nil, err
	}

	if len(sources) != len(query.SourceIds) {
		return nil, fmt.Errorf("some sources not found, source_ids=%v", query.SourceIds)
	}

	resp, err := b.impl.SimilaritySearchSourceDocs(ctx,
		&SimilaritySearchSourceDocsQuery{
			NotebookId: query.NotebookId,
			Query:      query.Target,
			SourceIds:  query.SourceIds,
			Count:      query.Count,
		})
	if err != nil {
		slog.ErrorContext(ctx, "similarity search source docs failed",
			slog.Any("err", err),
			slog.Any("source_ids", query.SourceIds),
		)

		return nil, err
	}

	return &AgentSearchSourceResult{
		Chunks: resp,
	}, nil
}
