package util

import (
	"strings"
	"unicode/utf8"
)

type ChunkByteSpan struct {
	StartByte int
	EndByte   int
}

// LocateChunkStart 以“有序分块”前提定位当前 chunk 在 source 中的起始位置。
// 定位策略：
// 1) 优先利用和前一个 chunk 的重叠关系直接反推；
// 2) 再做从前一命中点向前的扫描；
// 3) 最后降级为从 prevEnd 或全局索引查找。
func LocateChunkStart(
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

// BuildChunkByteSpans 按 chunks 的顺序批量定位 byte span。
// 返回值与 chunks 一一对应；无法定位时 span 为 {-1,-1}。
func BuildChunkByteSpans(source string, chunks []string) []ChunkByteSpan {
	spans := make([]ChunkByteSpan, len(chunks))
	if len(chunks) == 0 {
		return spans
	}

	for idx := range spans {
		spans[idx] = ChunkByteSpan{StartByte: -1, EndByte: -1}
	}

	var (
		prevChunk string
		prevStart int
		prevEnd   int
		hasPrev   bool
	)
	for idx, chunk := range chunks {
		if chunk == "" {
			continue
		}
		start, ok := LocateChunkStart(source, chunk, prevChunk, prevStart, prevEnd, hasPrev)
		if !ok {
			continue
		}
		end := start + len(chunk)
		if start < 0 || end > len(source) || end <= start {
			continue
		}

		spans[idx] = ChunkByteSpan{
			StartByte: start,
			EndByte:   end,
		}
		prevChunk = chunk
		prevStart = start
		prevEnd = end
		hasPrev = true
	}

	return spans
}

// BuildRuneIndexByByteOffset 将每个 byte offset 映射到 rune offset，
// 方便后续 O(1) 完成 byte->rune 转换。
func BuildRuneIndexByByteOffset(source string) []int {
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

// ByteOffsetToRuneOffset 通过预构建索引把 byte offset 转为 rune offset。
func ByteOffsetToRuneOffset(runeIndexByByteOffset []int, byteOffset int) int {
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

func locateChunkStartByOverlap(source, chunk, prevChunk string, prevEnd int) (int, bool) {
	if prevChunk == "" {
		return 0, false
	}

	overlap := longestSuffixPrefixOverlap(prevChunk, chunk)
	if overlap == 0 || overlap == len(chunk) {
		return 0, false
	}

	// 通过“前块后缀 = 当前块前缀”的重叠长度，直接反推当前位置。
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

		// 对于重叠分块，允许 candidateStart < prevEnd；
		// 但若 chunk 完全落在前一个 chunk 内，多半是重复文本误命中，继续前扫。
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
