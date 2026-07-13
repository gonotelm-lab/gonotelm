package agentize

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/source/errors"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/log"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/allegro/bigcache/v3"
	"github.com/bytedance/sonic"
)

// 给Agent使用的Service
type Service struct {
	config        Config
	soruceRepo    sourcerepo.Repository
	storageRepo   sourcerepo.StorageRepository
	sourceDocRepo sourcerepo.SourceDocRepository

	cache *bigcache.BigCache
}

type Config struct {
	CacheEviction time.Duration
	CacheMaxMB    int
}

func (c *Config) normalize() {
	if c.CacheEviction <= 0 {
		c.CacheEviction = time.Minute * 15
	}
	if c.CacheMaxMB <= 0 {
		c.CacheMaxMB = 256 // 256MB
	}
}

func NewService(
	config Config,
	soruceRepo sourcerepo.Repository,
	storageRepo sourcerepo.StorageRepository,
	sourceDocRepo sourcerepo.SourceDocRepository,
) *Service {
	config.normalize()

	s := &Service{
		config:        config,
		soruceRepo:    soruceRepo,
		storageRepo:   storageRepo,
		sourceDocRepo: sourceDocRepo,
	}

	bgc := bigcache.DefaultConfig(config.CacheEviction)
	bgc.Shards = 64
	bgc.HardMaxCacheSize = config.CacheMaxMB
	bgc.Logger = &log.BigcacheLogger{}
	bgc.Verbose = true
	cache, err := bigcache.New(context.Background(), bgc)
	if err != nil {
		slog.Warn("failed to create bigcache", slog.Any("err", err))
	}
	s.cache = cache

	return s
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

// GetSourceContent 获取来源全部内容
func (s *Service) GetSourceContent(ctx context.Context, id valobj.Id) ([]byte, error) {
	cacheKey := id.String()

	if cached, ok := s.loadFromCache(cacheKey); ok {
		return cached.Content, nil
	}

	rawContent, _, _, err := s.fetchAndSetCache(ctx, id)
	if err != nil {
		return nil, err
	}

	return rawContent, nil
}

type StatSourceResult struct {
	Bytes    int
	Runes    int
	Lines    int
	Abstract string
}

func (s *Service) StatSource(ctx context.Context, id valobj.Id) (*StatSourceResult, error) {
	if cached, ok := s.loadFromCache(id.String()); ok {
		return &StatSourceResult{
			Bytes:    len(cached.Content),
			Runes:    utf8.RuneCount(cached.Content),
			Lines:    len(cached.LineRanges),
			Abstract: cached.Abstract,
		}, nil
	}

	rawContent, lineRanges, abstract, err := s.fetchAndSetCache(ctx, id)
	if err != nil {
		return nil, err
	}

	return &StatSourceResult{
		Bytes:    len(rawContent),
		Runes:    utf8.RuneCount(rawContent),
		Lines:    len(lineRanges),
		Abstract: abstract,
	}, nil
}

type ReadSourceQuery struct {
	SourceId valobj.Id
	Offset   int // 起始行 1-indexed
	Limit    int // 读取的行数
}

type ReadSourceResult struct {
	TotalLines int
	Lines      []ReadSourceResultLine
}

type ReadSourceResultLine struct {
	LineNo int // 行号 1-indexed
	Line   []byte
}

// ReadSource 为 agent 提供来源的内容
func (s *Service) ReadSource(
	ctx context.Context,
	query *ReadSourceQuery,
) (*ReadSourceResult, error) {
	sourceID := query.SourceId.String()

	if s.cache != nil {
		if encodedPayload, err := s.cache.Get(sourceID); err == nil {
			cached, decodeErr := s.decodeCachedSource(encodedPayload)
			if decodeErr == nil {
				return s.selectContent(cached.Content, cached.LineRanges, query.Offset, query.Limit)
			}

			slog.WarnContext(ctx, "decode source cache failed, fallback to refetch",
				slog.Any("err", decodeErr),
				slog.String("source_id", sourceID),
			)
		}
	}

	rawContent, lineRanges, _, err := s.fetchAndSetCache(ctx, query.SourceId)
	if err != nil {
		return nil, err
	}

	return s.selectContent(rawContent, lineRanges, query.Offset, query.Limit)
}

func (s *Service) loadFromCache(cacheKey string) (*cachedSource, bool) {
	if s.cache == nil {
		return nil, false
	}

	encodedPayload, err := s.cache.Get(cacheKey)
	if err != nil {
		return nil, false
	}

	cached, decodeErr := s.decodeCachedSource(encodedPayload)
	if decodeErr != nil {
		return nil, false
	}

	return cached, true
}

func (s *Service) fetchAndSetCache(
	ctx context.Context,
	sourceId valobj.Id,
) ([]byte, []lineRange, string, error) {
	rawContent, abstract, err := s.fetchSourceContent(ctx, sourceId)
	if err != nil {
		return nil, nil, "", err
	}

	lineRanges := s.buildLineRanges(rawContent)
	if s.cache == nil {
		return rawContent, lineRanges, abstract, nil
	}

	newPayload, err := s.encodeCachedSource(rawContent, lineRanges, abstract)
	if err != nil {
		slog.ErrorContext(ctx, "encode source cache payload failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
		)

		return rawContent, lineRanges, abstract, err
	}

	if err := s.cache.Set(sourceId.String(), newPayload); err != nil {
		slog.ErrorContext(ctx, "set source cache failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
		)
	}

	return rawContent, lineRanges, abstract, nil
}

func (s *Service) encodeCachedSource(
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

func (s *Service) decodeCachedSource(payload []byte) (*cachedSource, error) {
	var c cachedSource
	if err := sonic.Unmarshal(payload, &c); err != nil {
		return nil, fmt.Errorf("unmarshal cache payload failed: %w", err)
	}

	return &c, nil
}

func (s *Service) selectContent(
	rawContent []byte,
	lineRanges []lineRange,
	offset int,
	limit int,
) (*ReadSourceResult, error) {
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

	result := &ReadSourceResult{
		TotalLines: totalLines,
		Lines:      make([]ReadSourceResultLine, 0, end-start),
	}
	contentLen := len(rawContent)
	for i := start; i < end; i++ {
		lineRange := lineRanges[i]
		if lineRange.Start < 0 || lineRange.End < lineRange.Start || lineRange.End > contentLen {
			return nil, fmt.Errorf("invalid line range at line %d: start=%d end=%d content=%d",
				i+1, lineRange.Start, lineRange.End, contentLen)
		}
		result.Lines = append(result.Lines, ReadSourceResultLine{
			LineNo: i + 1,
			Line:   rawContent[lineRange.Start:lineRange.End],
		})
	}

	return result, nil
}

func (s *Service) fetchSourceContent(
	ctx context.Context,
	sourceId valobj.Id,
) ([]byte, string, error) {
	src, err := s.soruceRepo.FindById(ctx, sourceId)
	if err != nil {
		if errors.Is(err, domainerr.ErrSourceNotFound) {
			return nil, "", fmt.Errorf("source not found, id=%s", sourceId)
		}

		slog.ErrorContext(ctx, "find source failed",
			slog.Any("err", err),
			slog.String("source_id", sourceId.String()),
		)

		return nil, "", fmt.Errorf("find source failed, id=%s, err=%w", sourceId, err)
	}

	if src.ParsedContentKey == "" {
		return nil, "", fmt.Errorf("source does not contain any valid parsed content key")
	}

	content, _, err := s.storageRepo.GetObject(ctx, src.ParsedContentKey)
	if err != nil {
		return nil, "", fmt.Errorf("get parsed content failed, id=%s, err=%w", sourceId, err)
	}

	return content, src.Abstract, nil
}

func (s *Service) buildLineRanges(rawContent []byte) []lineRange {
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

type SearchSourceQuery struct {
	NotebookId valobj.Id
	SourceIds  []valobj.Id
	Target     string
	Count      int
}

type SearchSourceResult struct {
	Chunks []*entity.SourceDoc
}

func (s *Service) SearchSource(
	ctx context.Context,
	query *SearchSourceQuery,
) (*SearchSourceResult, error) {
	sources, err := s.soruceRepo.GetByNotebookIdAndIds(ctx, query.NotebookId, query.SourceIds)
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

	chunks, err := s.sourceDocRepo.Query(ctx,
		&sourcerepo.SourceDocQueryParams{
			NotebookId: query.NotebookId,
			SourceId:   query.SourceIds,
			Target:     query.Target,
			Limit:      query.Count,
		})
	if err != nil {
		slog.ErrorContext(ctx, "query source docs failed",
			slog.Any("err", err),
			slog.Any("source_ids", query.SourceIds),
		)

		return nil, err
	}

	return &SearchSourceResult{
		Chunks: chunks,
	}, nil
}

func (s *Service) CheckSourceDocAllowAccess(
	ctx context.Context,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
	sourceDocIds []valobj.Id,
) error {
	targetDocs, err := s.sourceDocRepo.BatchFind(ctx, notebookId, uuid.EmptyUUID(), sourceDocIds)
	if err != nil {
		return fmt.Errorf("batch find source docs failed err=%w", err)
	}

	// 检查targetDocs的sourceId是否在sourceIds中
	for _, doc := range targetDocs {
		if !slices.Contains(sourceIds, doc.SourceId) {
			return fmt.Errorf(
				"source doc not allowed to access, source_id=%s, source_doc_id=%s",
				doc.SourceId.String(), doc.Id.String(),
			)
		}
	}

	return nil
}
