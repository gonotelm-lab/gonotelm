package markdown

import (
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	goldtext "github.com/yuin/goldmark/text"
)

type chunkDraft struct {
	content string
	meta    map[string]any
	br      byteRange
}

type topLevelItem struct {
	isHeading    bool
	isFencedCode bool
	level        int
	title        string
	br           byteRange
}

type section struct {
	heading *headingEntry
	blocks  []byteRange
}

func splitSource(source string, chunkSize int, overlap int, lenFn func(string) int) []chunkDraft {
	if source == "" {
		return nil
	}

	src := []byte(source)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(goldtext.NewReader(src))
	runeIdx := buildRuneIndex(source)

	items := collectTopLevelItems(doc, src)
	if len(items) == 0 {
		return nil
	}
	items = coalesceAdjacentBlocks(src, items)

	sections := buildSections(items)
	stack := newHeadingStack()

	var result []chunkDraft
	for _, sec := range sections {
		if sec.heading != nil {
			stack.push(*sec.heading)
		}
		result = append(result, packSection(src, sec, stack, runeIdx, chunkSize, overlap, lenFn)...)
	}

	return result
}

func buildSections(items []topLevelItem) []section {
	var sections []section
	var cur *section

	flushSection := func() {
		if cur == nil {
			return
		}
		if cur.heading != nil || len(cur.blocks) > 0 {
			sections = append(sections, *cur)
		}
		cur = nil
	}

	for _, item := range items {
		if item.isHeading {
			flushSection()
			entry := headingEntry{
				level: item.level,
				title: item.title,
				br:    item.br,
			}
			cur = &section{heading: &entry}
			continue
		}

		if cur == nil {
			cur = &section{}
		}
		cur.blocks = append(cur.blocks, item.br)
	}

	flushSection()
	return sections
}

func packSection(
	source []byte,
	sec section,
	stack *headingStack,
	runeIdx runeIndex,
	chunkSize int,
	overlap int,
	lenFn func(string) int,
) []chunkDraft {
	if len(sec.blocks) == 0 {
		if sec.heading == nil {
			return nil
		}
		return []chunkDraft{makeChunk(source, sec.heading.br, stack, runeIdx)}
	}

	groups := packBlockGroups(source, sec.blocks, chunkSize, overlap, lenFn)
	if len(groups) == 0 {
		return nil
	}

	chunks := make([]chunkDraft, 0, len(groups))
	for i, group := range groups {
		br, ok := mergeRangeList(group)
		if !ok {
			continue
		}
		if i == 0 && sec.heading != nil && br.start > sec.heading.br.start {
			br.start = sec.heading.br.start
		}
		chunks = append(chunks, makeChunk(source, br, stack, runeIdx))
	}

	return chunks
}

func packBlockGroups(source []byte, blocks []byteRange, chunkSize int, overlap int, lenFn func(string) int) [][]byteRange {
	var groups [][]byteRange
	var current []byteRange
	currentLen := 0

	flushCurrent := func() {
		if len(current) == 0 {
			return
		}
		groups = append(groups, current)
		current = nil
		currentLen = 0
	}

	for _, block := range blocks {
		blockLen := blockLength(source, block, lenFn)

		if blockLen > chunkSize {
			flushCurrent()
			groups = append(groups, []byteRange{block})
			continue
		}

		if currentLen > 0 && currentLen+blockLen > chunkSize {
			flushCurrent()
		}

		current = append(current, block)
		currentLen += blockLen
	}

	flushCurrent()
	return groups
}

func makeChunk(source []byte, br byteRange, stack *headingStack, runeIdx runeIndex) chunkDraft {
	content := string(source[br.start:br.end])
	meta := stack.toMetaData()
	setPositionMeta(
		meta,
		br.start,
		br.end,
		byteOffsetToRune(runeIdx, br.start),
		byteOffsetToRune(runeIdx, br.end),
	)
	return chunkDraft{content: content, meta: meta, br: br}
}

func collectTopLevelItems(doc ast.Node, source []byte) []topLevelItem {
	nodes := make([]ast.Node, 0)
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		nodes = append(nodes, n)
	}

	nextLineStart := func(i int) int {
		for j := i + 1; j < len(nodes); j++ {
			if _, isHeading := nodes[j].(*ast.Heading); isHeading {
				return -1
			}
			if _, isFence := nodes[j].(*ast.FencedCodeBlock); isFence {
				return -1
			}
			if br, ok := extractNodeByteRange(nodes[j], source); ok {
				return extendToLineStart(source, br.start)
			}
			if childStart := findFirstBlockStart(nodes[j], source); childStart >= 0 {
				return extendToLineStart(source, childStart)
			}
		}
		return -1
	}

	items := make([]topLevelItem, 0, len(nodes))
	lastItemEnd := 0
	for i, node := range nodes {
		switch n := node.(type) {
		case *ast.Heading:
			br, ok := extractBlockByteRange(n, source, -1)
			if !ok {
				continue
			}
			items = append(items, topLevelItem{
				isHeading: true,
				level:     n.Level,
				title:     extractHeadingTitle(source, br),
				br:        br,
			})
			lastItemEnd = br.end
		case *ast.ThematicBreak:
			if br, ok := extractThematicBreakRange(n, source); ok {
				items = append(items, topLevelItem{br: br})
				lastItemEnd = br.end
			} else if br, ok := findThematicBreakInSource(source, lastItemEnd); ok {
				items = append(items, topLevelItem{br: br})
				lastItemEnd = br.end
			}
		case *ast.FencedCodeBlock:
			br, ok := extractBlockByteRange(n, source, -1)
			if !ok {
				continue
			}
			items = append(items, topLevelItem{br: br, isFencedCode: true})
			lastItemEnd = br.end
		default:
			nextStart := nextLineStart(i)
			br, ok := extractBlockByteRange(node, source, nextStart)
			if !ok {
				continue
			}
			if strings.TrimSpace(string(source[br.start:br.end])) == "" {
				continue
			}
			items = append(items, topLevelItem{br: br})
			lastItemEnd = br.end
		}
	}

	return items
}

// coalesceAdjacentBlocks 合并应作为整体切分的相邻块，例如 ``` opener 段落 + FencedCodeBlock 正文。
func coalesceAdjacentBlocks(source []byte, items []topLevelItem) []topLevelItem {
	if len(items) <= 1 {
		return items
	}

	out := make([]topLevelItem, 0, len(items))
	for i := 0; i < len(items); i++ {
		item := items[i]
		if item.isHeading {
			out = append(out, item)
			continue
		}

		if len(out) > 0 && !out[len(out)-1].isHeading {
			prev := &out[len(out)-1]
			if shouldMergeBlocks(source, *prev, item) {
				prev.br.end = item.br.end
				continue
			}
		}

		out = append(out, item)
	}

	return out
}

func shouldMergeBlocks(source []byte, prev, cur topLevelItem) bool {
	if cur.isFencedCode {
		return false
	}

	prevText := string(source[prev.br.start:prev.br.end])
	curText := string(source[cur.br.start:cur.br.end])

	if strings.Contains(prevText, "```") || strings.Contains(prevText, "~~~") {
		trimmed := strings.TrimSpace(prevText)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") || strings.HasSuffix(trimmed, "```") || strings.HasSuffix(trimmed, "~~~") {
			return true
		}
		// 段落以 opening fence 行结尾，如 "text:\n\n```mermaid"
		lastLine := prevText
		if idx := strings.LastIndex(prevText, "\n"); idx >= 0 {
			lastLine = prevText[idx+1:]
		}
		lastLine = strings.TrimSpace(lastLine)
		if strings.HasPrefix(lastLine, "```") || strings.HasPrefix(lastLine, "~~~") {
			return true
		}
	}

	// 代码块 closing fence 独立成段时并回前块
	curTrimmed := strings.TrimSpace(curText)
	if (strings.HasPrefix(curTrimmed, "```") || strings.HasPrefix(curTrimmed, "~~~")) && len(curTrimmed) <= 6 {
		return strings.Contains(prevText, "```") || strings.Contains(prevText, "graph ")
	}

	_ = curText
	return false
}

func extractThematicBreakRange(node ast.Node, source []byte) (byteRange, bool) {
	if br, ok := extractNodeByteRange(node, source); ok {
		br.start = extendToLineStart(source, br.start)
		br.end = extendToLineEnd(source, br.end)
		return br, true
	}

	// goldmark 有时不给 ThematicBreak Lines()，用 RawText 或相邻段落边界不可靠，跳过即可。
	return byteRange{}, false
}
