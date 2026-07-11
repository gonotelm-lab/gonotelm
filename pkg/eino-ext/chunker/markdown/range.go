package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark/ast"
)

type byteRange struct {
	start, end int
}

func extractNodeByteRange(node ast.Node, source []byte) (byteRange, bool) {
	if node == nil || len(source) == 0 {
		return byteRange{}, false
	}

	lines := node.Lines()
	if lines == nil || lines.Len() == 0 {
		return byteRange{}, false
	}

	start := lines.At(0).Start
	end := lines.At(lines.Len() - 1).Stop
	if start < 0 || end > len(source) || start >= end {
		return byteRange{}, false
	}

	return byteRange{start: start, end: end}, true
}

func extendToLineStart(source []byte, pos int) int {
	for pos > 0 && source[pos-1] != '\n' {
		pos--
	}
	return pos
}

func extendToLineEnd(source []byte, pos int) int {
	for pos < len(source) && source[pos] != '\n' {
		pos++
	}
	if pos < len(source) {
		pos++
	}
	return pos
}

func extractBlockByteRange(node ast.Node, source []byte, nextLineStart int) (byteRange, bool) {
	switch n := node.(type) {
	case *ast.Heading:
		br, ok := extractNodeByteRange(n, source)
		if !ok {
			return byteRange{}, false
		}
		br.start = extendToLineStart(source, br.start)
		br.end = extendToLineEnd(source, br.end)
		return br, true
	case *ast.FencedCodeBlock:
		return extractFencedCodeBlockRange(n, source)
	case *ast.Blockquote, *ast.List:
		return extractContainerBlockRange(node, source)
	default:
		br, ok := extractNodeByteRange(node, source)
		if !ok {
			return byteRange{}, false
		}
		br.end = extendToLineEnd(source, br.end)
		if nextLineStart >= 0 && nextLineStart > br.end && nextLineStart <= len(source) {
			br.end = nextLineStart
		}
		return br, true
	}
}

func extractFencedCodeBlockRange(node *ast.FencedCodeBlock, source []byte) (byteRange, bool) {
	br, ok := extractNodeByteRange(node, source)
	if !ok {
		return byteRange{}, false
	}

	br.start = findOpeningFenceStart(source, br.start)

	lineEnd := br.end
	for lineEnd < len(source) {
		nextLineEnd := extendToLineEnd(source, lineEnd)
		line := strings.TrimSpace(string(source[lineEnd:nextLineEnd]))
		if line == "" {
			lineEnd = nextLineEnd
			continue
		}
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			br.end = nextLineEnd
		}
		break
	}

	return br, true
}

func findOpeningFenceStart(source []byte, blockStart int) int {
	pos := blockStart
	for pos > 0 {
		lineStart := extendToLineStart(source, pos-1)
		lineEnd := extendToLineEnd(source, lineStart)
		line := strings.TrimSpace(string(source[lineStart:lineEnd]))
		if strings.HasPrefix(line, "```") || strings.HasPrefix(line, "~~~") {
			return lineStart
		}
		if line != "" {
			return blockStart
		}
		if lineStart == 0 {
			return blockStart
		}
		pos = lineStart
	}
	return blockStart
}

func extractContainerBlockRange(node ast.Node, source []byte) (byteRange, bool) {
	var ranges []byteRange
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || n.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		if br, ok := extractNodeByteRange(n, source); ok {
			ranges = append(ranges, br)
		}
		return ast.WalkContinue, nil
	})
	br, ok := mergeRangeList(ranges)
	if !ok {
		return byteRange{}, false
	}
	br.start = extendToLineStart(source, br.start)
	br.end = extendToLineEnd(source, br.end)
	return br, true
}

func mergeRanges(source []byte, ranges []byteRange) string {
	if len(ranges) == 0 {
		return ""
	}
	return string(source[ranges[0].start:ranges[len(ranges)-1].end])
}

func mergeRangeList(ranges []byteRange) (byteRange, bool) {
	if len(ranges) == 0 {
		return byteRange{}, false
	}
	return byteRange{
		start: ranges[0].start,
		end:   ranges[len(ranges)-1].end,
	}, true
}

type runeIndex struct {
	byteOffsets []int
	runeOffsets []int
}

func buildRuneIndex(source string) runeIndex {
	byteOffsets := make([]int, 0, utf8.RuneCountInString(source)+1)
	runeOffsets := make([]int, 0, cap(byteOffsets))

	byteOffsets = append(byteOffsets, 0)
	runeOffsets = append(runeOffsets, 0)

	runeCount := 0
	for i := 0; i < len(source); {
		_, size := utf8.DecodeRuneInString(source[i:])
		if size == 0 {
			break
		}
		i += size
		runeCount++
		byteOffsets = append(byteOffsets, i)
		runeOffsets = append(runeOffsets, runeCount)
	}

	return runeIndex{byteOffsets: byteOffsets, runeOffsets: runeOffsets}
}

func byteOffsetToRune(idx runeIndex, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}

	lo, hi := 0, len(idx.byteOffsets)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		switch {
		case idx.byteOffsets[mid] == byteOffset:
			return idx.runeOffsets[mid]
		case idx.byteOffsets[mid] < byteOffset:
			lo = mid + 1
		default:
			hi = mid - 1
		}
	}

	if hi < 0 {
		return 0
	}
	return idx.runeOffsets[hi]
}

func findFirstBlockStart(node ast.Node, source []byte) int {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if br, ok := extractNodeByteRange(c, source); ok {
			return br.start
		}
		if start := findFirstBlockStart(c, source); start >= 0 {
			return start
		}
	}
	return -1
}

func findThematicBreakInSource(source []byte, searchStart int) (byteRange, bool) {
	pos := searchStart
	for pos < len(source) {
		lineEnd := extendToLineEnd(source, pos)
		line := strings.TrimSpace(string(source[pos:lineEnd]))
		if line != "" {
			if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "***") || strings.HasPrefix(line, "___") {
				return byteRange{start: extendToLineStart(source, pos), end: lineEnd}, true
			}
			return byteRange{}, false
		}
		pos = lineEnd
	}
	return byteRange{}, false
}

func extractHeadingTitle(source []byte, br byteRange) string {
	line := string(source[br.start:br.end])
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	return strings.TrimSpace(line)
}

func blockLength(source []byte, br byteRange, lenFn func(string) int) int {
	if lenFn == nil {
		return br.end - br.start
	}
	return lenFn(string(source[br.start:br.end]))
}
