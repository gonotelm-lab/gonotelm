package recursive

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)

type KeepType uint8

const (
	// KeepTypeNone 表示分句后丢弃分隔符。
	KeepTypeNone KeepType = iota
	// KeepTypeStart 表示分隔符保留在句子单元开头。
	KeepTypeStart
	// KeepTypeEnd 表示分隔符保留在句子单元结尾。
	KeepTypeEnd
)

// IDGenerator 用于为切分后的 chunk 生成新 ID。
type IDGenerator func(ctx context.Context, originalID string, splitIndex int) string

type Config struct {
	ChunkSize int
	// OverlapSize 是相邻 chunk 重叠长度的上限。
	// 实际重叠会对齐到完整句子单元，因此可能小于该值。
	OverlapSize int
	// Separators 按顺序用于递归分句。
	// chunk 只会在句子单元边界处切分，不会从单元内部切开。
	Separators []string
	// LenFunc 用于计算长度，默认使用 len。
	LenFunc func(string) int
	// KeepType 控制分隔符在句子单元中的保留方式。
	KeepType KeepType
	// IDGenerator 可选，用于为切分后的 chunk 生成新 ID。
	IDGenerator IDGenerator
}

func defaultIDGenerator(ctx context.Context, originalID string, _ int) string {
	return originalID
}

// NewSplitter 创建一个“句子优先”的递归切分器。
func NewSplitter(ctx context.Context, config *Config) (document.Transformer, error) {
	if config == nil {
		return nil, fmt.Errorf("config must not be nil")
	}
	if config.ChunkSize <= 0 {
		return nil, fmt.Errorf("chunk size must be greater than zero")
	}
	if config.OverlapSize < 0 {
		return nil, fmt.Errorf("overlap must be greater than or equal to zero")
	}
	if config.KeepType != KeepTypeNone && config.KeepType != KeepTypeStart && config.KeepType != KeepTypeEnd {
		return nil, fmt.Errorf("unknown keep type: %v", config.KeepType)
	}

	lenFunc := config.LenFunc
	if lenFunc == nil {
		lenFunc = func(s string) int { return len(s) }
	}

	separators := config.Separators
	if len(separators) == 0 {
		separators = []string{"\n\n", "\n", ". ", "? ", "! ", "。", "？", "！"}
	}

	idGenerator := config.IDGenerator
	if idGenerator == nil {
		idGenerator = defaultIDGenerator
	}

	return &recursiveChunker{
		chunkSize:   config.ChunkSize,
		overlapSize: config.OverlapSize,
		separators:  separators,
		lenFunc:     lenFunc,
		keepType:    config.KeepType,
		idGenerator: idGenerator,
	}, nil
}

type recursiveChunker struct {
	chunkSize   int
	overlapSize int
	separators  []string
	lenFunc     func(string) int
	keepType    KeepType
	idGenerator IDGenerator
}

type textUnit struct {
	length int
	start  int
	end    int
}

var _ document.Transformer = (*recursiveChunker)(nil)

func (c *recursiveChunker) Transform(
	ctx context.Context,
	docs []*schema.Document,
	opts ...document.TransformerOption,
) ([]*schema.Document, error) {
	ret := make([]*schema.Document, 0, len(docs))

	for _, doc := range docs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if doc == nil {
			continue
		}

		chunks := c.splitText(doc.Content)

		for i, chunk := range chunks {
			ret = append(ret, &schema.Document{
				ID:       c.idGenerator(ctx, doc.ID, i),
				Content:  chunk,
				MetaData: deepCopyMap(doc.MetaData),
			})
		}
	}

	return ret, nil
}

func (c *recursiveChunker) splitText(text string) []string {
	if text == "" {
		return nil
	}

	units := c.splitToUnits(text, 0, len(text), c.separators)

	return c.packUnits(text, units)
}

func (c *recursiveChunker) splitToUnits(
	source string,
	start int,
	end int,
	separators []string,
) []textUnit {
	if start >= end {
		return nil
	}

	// 按分隔符优先级递归切分：
	// 命中某一级分隔符后，仅对超限片段递归到更细粒度分隔符。
	for idx, separator := range separators {
		if separator == "" || !strings.Contains(source[start:end], separator) {
			continue
		}

		parts := c.splitBySeparator(source, start, end, separator)
		units := make([]textUnit, 0, len(parts))
		nextSeparators := separators[idx+1:]

		for _, part := range parts {
			if part.start >= part.end {
				continue
			}
			if part.length <= c.chunkSize {
				units = append(units, part)
				continue
			}
			if len(nextSeparators) == 0 {
				units = append(units, c.splitOversizedUnit(source, part.start, part.end)...)
				continue
			}

			nestedUnits := c.splitToUnits(source, part.start, part.end, nextSeparators)
			units = append(units, nestedUnits...)
		}

		if len(units) > 0 {
			return units
		}
	}

	unit := c.newTextUnit(source, start, end)
	if unit.length <= c.chunkSize {
		return []textUnit{unit}
	}

	return c.splitOversizedUnit(source, start, end)
}

func (c *recursiveChunker) splitBySeparator(
	source string,
	start int,
	end int,
	separator string,
) []textUnit {
	switch c.keepType {
	case KeepTypeNone:
		return c.splitNone(source, start, end, separator)
	case KeepTypeStart:
		return c.splitStart(source, start, end, separator)
	case KeepTypeEnd:
		return c.splitEnd(source, start, end, separator)
	default:
		panic(fmt.Sprintf("unknown keep type: %v", c.keepType))
	}
}

func (c *recursiveChunker) splitEnd(
	source string,
	start int,
	end int,
	separator string,
) []textUnit {
	units := make([]textUnit, 0, strings.Count(source[start:end], separator)+1)
	unitStart := start

	for unitStart < end {
		idx := strings.Index(source[unitStart:end], separator)
		if idx < 0 {
			break
		}

		unitEnd := unitStart + idx + len(separator)
		units = append(units, c.newTextUnit(source, unitStart, unitEnd))
		unitStart = unitEnd
	}

	if unitStart < end {
		units = append(units, c.newTextUnit(source, unitStart, end))
	}

	return units
}

func (c *recursiveChunker) splitStart(
	source string,
	start int,
	end int,
	separator string,
) []textUnit {
	units := make([]textUnit, 0, strings.Count(source[start:end], separator)+1)
	unitStart := start

	for {
		idx := strings.Index(source[unitStart:end], separator)
		if idx < 0 {
			break
		}

		sepStart := unitStart + idx
		if sepStart > unitStart {
			units = append(units, c.newTextUnit(source, unitStart, sepStart))
		}
		unitStart = sepStart

		nextSearchStart := unitStart + len(separator)
		if nextSearchStart >= end {
			break
		}
		nextIdx := strings.Index(source[nextSearchStart:end], separator)
		if nextIdx < 0 {
			break
		}

		unitEnd := nextSearchStart + nextIdx
		units = append(units, c.newTextUnit(source, unitStart, unitEnd))
		unitStart = unitEnd
	}

	if unitStart < end {
		units = append(units, c.newTextUnit(source, unitStart, end))
	}

	return units
}

func (c *recursiveChunker) splitNone(
	source string,
	start int,
	end int,
	separator string,
) []textUnit {
	units := make([]textUnit, 0, strings.Count(source[start:end], separator)+1)
	unitStart := start

	for unitStart <= end {
		idx := strings.Index(source[unitStart:end], separator)
		if idx < 0 {
			break
		}

		unitEnd := unitStart + idx
		if unitEnd > unitStart {
			units = append(units, c.newTextUnit(source, unitStart, unitEnd))
		}
		unitStart = unitEnd + len(separator)
	}

	if unitStart < end {
		units = append(units, c.newTextUnit(source, unitStart, end))
	}

	return units
}

func (c *recursiveChunker) newTextUnit(
	source string,
	start int,
	end int,
) textUnit {
	return textUnit{
		length: c.lenFunc(source[start:end]),
		start:  start,
		end:    end,
	}
}

func (c *recursiveChunker) splitOversizedUnit(
	source string,
	start int,
	end int,
) []textUnit {
	if start >= end {
		return nil
	}

	units := make([]textUnit, 0)
	for left := start; left < end; {
		right := c.maxChunkEndByLen(source, left, end)
		if right <= left {
			// 在 lenFunc 约束无法容纳任意完整 rune 时，至少向前推进一个 rune，避免死循环。
			right = nextRuneBoundary(source, left, end)
		}

		units = append(units, c.newTextUnit(source, left, right))
		left = right
	}

	return units
}

func (c *recursiveChunker) maxChunkEndByLen(source string, start int, end int) int {
	boundaries := runeBoundaries(source, start, end)
	lo, hi := 1, len(boundaries)-1
	best := start

	for lo <= hi {
		mid := lo + (hi-lo)/2
		candidateEnd := boundaries[mid]
		candidateLen := c.lenFunc(source[start:candidateEnd])
		if candidateLen <= c.chunkSize {
			best = candidateEnd
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}

	return best
}

func runeBoundaries(source string, start int, end int) []int {
	bounds := make([]int, 0, end-start+1)
	bounds = append(bounds, start)
	for idx := range source[start:end] {
		if idx == 0 {
			continue
		}
		bounds = append(bounds, start+idx)
	}
	bounds = append(bounds, end)
	return bounds
}

func nextRuneBoundary(source string, start int, end int) int {
	for idx := range source[start:end] {
		if idx == 0 {
			continue
		}
		return start + idx
	}
	return end
}

func (c *recursiveChunker) packUnits(
	source string,
	units []textUnit,
) []string {
	if len(units) == 0 {
		return nil
	}

	totalLen := 0
	for _, unit := range units {
		totalLen += unit.length
	}

	// 滑动窗口打包策略：
	// 1) 当前窗口尽量扩展到不超过 chunkSize；
	// 2) 下一窗口从可行的重叠句子边界开始。
	chunks := make([]string, 0, c.estimateChunkCapacity(totalLen, len(units)))
	for start := 0; start < len(units); {
		end := start
		total := 0
		for end < len(units) {
			nextTotal := total + units[end].length
			if total > 0 && nextTotal > c.chunkSize {
				break
			}
			total = nextTotal
			end++
		}
		if end == start {
			end++
		}

		chunks = append(chunks, c.joinUnits(source, units[start:end], total))
		if end >= len(units) {
			break
		}

		nextStart := c.nextWindowStart(start, end, units[end].length, units)
		if nextStart <= start {
			nextStart = end
		}
		start = nextStart
	}

	return chunks
}

func (c *recursiveChunker) estimateChunkCapacity(totalLen, unitCount int) int {
	if unitCount <= 0 {
		return 0
	}

	step := c.chunkSize - c.overlapSize
	if step <= 0 {
		step = c.chunkSize
	}
	if step <= 0 {
		return unitCount
	}

	// 这里做一个保守估算，减少 chunks 切片扩容次数。
	estimated := totalLen/step + 2
	if estimated < 1 {
		return 1
	}
	if estimated > unitCount {
		return unitCount
	}
	return estimated
}

func (c *recursiveChunker) joinUnits(source string, units []textUnit, totalLen int) string {
	if len(units) == 0 {
		return ""
	}
	if c.keepType != KeepTypeNone {
		// 在 KeepTypeStart/End 场景下，units 对应连续原文区间，
		// 可直接切原文，避免 strings.Builder 的额外分配。
		start := units[0].start
		end := units[len(units)-1].end
		if start >= 0 && end >= start && end <= len(source) {
			return source[start:end]
		}
	}

	var builder strings.Builder
	builder.Grow(totalLen)
	for _, unit := range units {
		builder.WriteString(source[unit.start:unit.end])
	}
	return builder.String()
}

func (c *recursiveChunker) nextWindowStart(start, end, nextUnitLen int, units []textUnit) int {
	if c.overlapSize <= 0 || nextUnitLen > c.chunkSize {
		return end
	}

	// 从窗口尾部向前回退，寻找满足两条约束的最大重叠：
	// 1) 重叠长度不超过 overlapSize；
	// 2) 重叠 + 下一句长度不超过 chunkSize。
	overlapLen := 0
	nextStart := end
	for i := end - 1; i >= start; i-- {
		candidateOverlap := overlapLen + units[i].length
		if candidateOverlap > c.overlapSize {
			break
		}
		if candidateOverlap+nextUnitLen > c.chunkSize {
			break
		}

		overlapLen = candidateOverlap
		nextStart = i
	}

	return nextStart
}

func (c *recursiveChunker) GetType() string {
	return "RecursiveChunker"
}

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}

	ret := make(map[string]any, len(m))
	maps.Copy(ret, m)
	return ret
}
