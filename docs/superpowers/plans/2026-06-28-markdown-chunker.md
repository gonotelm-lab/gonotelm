# Markdown AST Chunker Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 `pkg/eino-ext/chunker/markdown` 实现基于 goldmark AST 的 `document.Transformer`，按 Heading Section 结构切分 markdown 并注入自定义元数据。

**Architecture:** 单次遍历 goldmark 顶层 AST 节点，维护 heading 栈与 section block 列表；超 ChunkSize 时在块边界 flush。chunk content 为 `source[byte_start:byte_end]` 原切片。

**Tech Stack:** Go, goldmark v1.8.2, cloudwego/eino document.Transformer

**Spec:** `docs/superpowers/specs/2026-06-28-markdown-chunker-design.md`

---

## File Structure

| File | 职责 |
|---|---|
| `pkg/eino-ext/chunker/markdown/metadata.go` | 元数据 key 常量、heading 栈、元数据注入 |
| `pkg/eino-ext/chunker/markdown/range.go` | byte range 提取、合并、rune index |
| `pkg/eino-ext/chunker/markdown/split.go` | AST 遍历 + section 打包核心逻辑 |
| `pkg/eino-ext/chunker/markdown/markdown.go` | Config、NewSplitter、Transform 入口 |
| `pkg/eino-ext/chunker/markdown/markdown_test.go` | 全部单元测试 |
| `pkg/eino-ext/chunker/markdown/testdata/sample.md` | 测试 fixture |

---

### Task 1: 元数据常量与 heading 栈

**Files:**
- Create: `pkg/eino-ext/chunker/markdown/metadata.go`
- Test: `pkg/eino-ext/chunker/markdown/markdown_test.go`

- [ ] **Step 1: Write the failing test**

```go
// markdown_test.go
package markdown

import "testing"

func TestHeadingStack_FillsMetadata(t *testing.T) {
	stack := newHeadingStack()
	stack.push(headingEntry{level: 1, title: "Rust 基础"})
	stack.push(headingEntry{level: 2, title: "所有权"})

	meta := stack.toMetaData()
	if meta[MetaHeadingH1Key] != "Rust 基础" {
		t.Fatalf("h1 = %q, want Rust 基础", meta[MetaHeadingH1Key])
	}
	if meta[MetaHeadingH2Key] != "所有权" {
		t.Fatalf("h2 = %q, want 所有权", meta[MetaHeadingH2Key])
	}
	if _, ok := meta[MetaHeadingH3Key]; ok {
		t.Fatal("h3 should not be set")
	}
}

func TestHeadingStack_PopSameLevel(t *testing.T) {
	stack := newHeadingStack()
	stack.push(headingEntry{level: 1, title: "A"})
	stack.push(headingEntry{level: 2, title: "B"})
	stack.push(headingEntry{level: 2, title: "C"})

	meta := stack.toMetaData()
	if meta[MetaHeadingH2Key] != "C" {
		t.Fatalf("h2 = %q, want C", meta[MetaHeadingH2Key])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run TestHeadingStack -v`
Expected: FAIL — package or types not found

- [ ] **Step 3: Write minimal implementation**

```go
// metadata.go
package markdown

const (
	MetaHeadingH1Key = "md_h1"
	MetaHeadingH2Key = "md_h2"
	MetaHeadingH3Key = "md_h3"
	MetaHeadingH4Key = "md_h4"
	MetaHeadingH5Key = "md_h5"
	MetaHeadingH6Key = "md_h6"

	MetaChunkByteStartKey = "md_chunk_byte_start"
	MetaChunkByteEndKey   = "md_chunk_byte_end"
	MetaChunkRuneStartKey = "md_chunk_rune_start"
	MetaChunkRuneEndKey   = "md_chunk_rune_end"
)

var headingMetaKeys = [6]string{
	MetaHeadingH1Key, MetaHeadingH2Key, MetaHeadingH3Key,
	MetaHeadingH4Key, MetaHeadingH5Key, MetaHeadingH6Key,
}

type headingEntry struct {
	level int
	title string
	br    byteRange
}

type headingStack struct {
	entries []headingEntry
}

func newHeadingStack() *headingStack {
	return &headingStack{}
}

func (s *headingStack) push(e headingEntry) {
	for len(s.entries) > 0 && s.entries[len(s.entries)-1].level >= e.level {
		s.entries = s.entries[:len(s.entries)-1]
	}
	s.entries = append(s.entries, e)
}

func (s *headingStack) toMetaData() map[string]any {
	meta := make(map[string]any)
	for _, e := range s.entries {
		if e.level < 1 || e.level > 6 {
			continue
		}
		meta[headingMetaKeys[e.level-1]] = e.title
	}
	return meta
}

func setPositionMeta(meta map[string]any, byteStart, byteEnd, runeStart, runeEnd int) {
	if meta == nil {
		return
	}
	meta[MetaChunkByteStartKey] = byteStart
	meta[MetaChunkByteEndKey] = byteEnd
	meta[MetaChunkRuneStartKey] = runeStart
	meta[MetaChunkRuneEndKey] = runeEnd
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run TestHeadingStack -v`
Expected: PASS

---

### Task 2: byte range 提取与 rune index

**Files:**
- Create: `pkg/eino-ext/chunker/markdown/range.go`
- Test: `pkg/eino-ext/chunker/markdown/markdown_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestExtractNodeByteRange(t *testing.T) {
	source := []byte("# Title\n\nParagraph one.\n")
	parser := goldmark.DefaultParser()
	doc := parser.Parse(text.NewReader(source))

	var heading *ast.Heading
	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		if h, ok := n.(*ast.Heading); ok {
			heading = h
			break
		}
	}
	if heading == nil {
		t.Fatal("heading not found")
	}

	br, ok := extractNodeByteRange(heading, source)
	if !ok {
		t.Fatal("expected ok")
	}
	if got := string(source[br.start:br.end]); got != "# Title\n" {
		t.Fatalf("got %q", got)
	}
}

func TestBuildRuneIndex(t *testing.T) {
	source := "你好world"
	idx := buildRuneIndex(source)
	if got := byteOffsetToRune(idx, 0); got != 0 {
		t.Fatalf("rune at 0 = %d, want 0", got)
	}
	if got := byteOffsetToRune(idx, len("你好")); got != 2 {
		t.Fatalf("rune after 你好 = %d, want 2", got)
	}
}

func TestMergeRanges(t *testing.T) {
	source := []byte("aaa\n\nbbb")
	ranges := []byteRange{{0, 3}, {5, 8}}
	got := mergeRanges(source, ranges)
	if got != "aaa\n\nbbb" {
		t.Fatalf("got %q", got)
	}
}
```

Add imports in test file:
```go
import (
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run "TestExtract|TestBuildRune|TestMerge" -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// range.go
package markdown

import (
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

func mergeRanges(source []byte, ranges []byteRange) string {
	if len(ranges) == 0 {
		return ""
	}
	return string(source[ranges[0].start : ranges[len(ranges)-1].end])
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
	// binary search in byteOffsets
	lo, hi := 0, len(idx.byteOffsets)-1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		if idx.byteOffsets[mid] == byteOffset {
			return idx.runeOffsets[mid]
		}
		if idx.byteOffsets[mid] < byteOffset {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if hi < 0 {
		return 0
	}
	return idx.runeOffsets[hi]
}

func extractHeadingTitle(source []byte, br byteRange) string {
	line := string(source[br.start:br.end])
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	return strings.TrimSpace(line)
}
```

Add `"strings"` import to range.go.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run "TestExtract|TestBuildRune|TestMerge" -v`
Expected: PASS

---

### Task 3: Section 打包核心逻辑

**Files:**
- Create: `pkg/eino-ext/chunker/markdown/split.go`
- Test: `pkg/eino-ext/chunker/markdown/markdown_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestSplitSource_HeadingSections(t *testing.T) {
	source := "# Rust 基础\n\n所有权系统。\n\n## 借用\n\n借用规则。\n"
	chunks := splitSource(source, 500, func(s string) int { return len(s) })

	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].content != "# Rust 基础\n\n所有权系统。\n" {
		t.Fatalf("chunk0 content = %q", chunks[0].content)
	}
	if chunks[0].meta[MetaHeadingH1Key] != "Rust 基础" {
		t.Fatalf("chunk0 h1 = %v", chunks[0].meta[MetaHeadingH1Key])
	}
	if chunks[1].meta[MetaHeadingH2Key] != "借用" {
		t.Fatalf("chunk1 h2 = %v", chunks[1].meta[MetaHeadingH2Key])
	}
}

func TestSplitSource_OversizedSectionByBlock(t *testing.T) {
	para1 := strings.Repeat("A", 300)
	para2 := strings.Repeat("B", 300)
	source := "# Title\n\n" + para1 + "\n\n" + para2 + "\n"
	chunks := splitSource(source, 350, func(s string) int { return len(s) })

	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}
	for _, c := range chunks {
		if strings.Contains(c.content, "A") && strings.Contains(c.content, "B") {
			if len(c.content) > 350+len("# Title\n\n") {
				t.Fatalf("chunk should not merge both oversized paragraphs: len=%d", len(c.content))
			}
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run TestSplitSource -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// split.go
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

func splitSource(source string, chunkSize int, lenFn func(string) int) []chunkDraft {
	if source == "" {
		return nil
	}
	src := []byte(source)
	parser := goldmark.DefaultParser()
	doc := parser.Parse(goldtext.NewReader(src))
	runeIdx := buildRuneIndex(source)

	var result []chunkDraft
	stack := newHeadingStack()

	var curHeading *headingEntry
	var blocks []byteRange
	blocksLen := 0

	flush := func(includeHeading bool) {
		if len(blocks) == 0 {
			return
		}
		br, ok := mergeRangeList(blocks)
		if !ok {
			blocks = blocks[:0]
			blocksLen = 0
			return
		}
		if includeHeading && curHeading != nil {
			br.start = curHeading.br.start
		}
		content := string(src[br.start:br.end])
		meta := stack.toMetaData()
		setPositionMeta(meta,
			br.start, br.end,
			byteOffsetToRune(runeIdx, br.start),
			byteOffsetToRune(runeIdx, br.end),
		)
		result = append(result, chunkDraft{content: content, meta: meta, br: br})
		blocks = blocks[:0]
		blocksLen = 0
	}

	flushWithBlockSplit := func() {
		if len(blocks) == 0 {
			return
		}
		first := true
		groupStart := 0
		groupLen := 0

		for i, b := range blocks {
			blockContent := string(src[b.start:b.end])
			blockLen := lenFn(blockContent)

			if groupLen > 0 && groupLen+blockLen > chunkSize {
				group := blocks[groupStart:i]
				br, ok := mergeRangeList(group)
				if ok {
					if first && curHeading != nil {
						br.start = curHeading.br.start
						first = false
					}
					content := string(src[br.start:br.end])
					meta := stack.toMetaData()
					setPositionMeta(meta,
						br.start, br.end,
						byteOffsetToRune(runeIdx, br.start),
						byteOffsetToRune(runeIdx, br.end),
					)
					result = append(result, chunkDraft{content: content, meta: meta, br: br})
				}
				groupStart = i
				groupLen = 0
			}
			groupLen += blockLen
		}

		if groupStart < len(blocks) {
			group := blocks[groupStart:]
			br, ok := mergeRangeList(group)
			if ok {
				if first && curHeading != nil {
					br.start = curHeading.br.start
				}
				content := string(src[br.start:br.end])
				meta := stack.toMetaData()
				setPositionMeta(meta,
					br.start, br.end,
					byteOffsetToRune(runeIdx, br.start),
					byteOffsetToRune(runeIdx, br.end),
				)
				result = append(result, chunkDraft{content: content, meta: meta, br: br})
			}
		}
		blocks = blocks[:0]
		blocksLen = 0
	}

	for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
		switch node := n.(type) {
		case *ast.Heading:
			if blocksLen > chunkSize {
				flushWithBlockSplit()
			} else {
				flush(true)
			}
			br, ok := extractNodeByteRange(node, src)
			if !ok {
				continue
			}
			entry := headingEntry{
				level: node.Level,
				title: extractHeadingTitle(src, br),
				br:    br,
			}
			stack.push(entry)
			curHeading = &entry
		default:
			br, ok := extractNodeByteRange(node, src)
			if !ok {
				continue
			}
			blockContent := string(src[br.start:br.end])
			if strings.TrimSpace(blockContent) == "" {
				continue
			}
			blocks = append(blocks, br)
			blocksLen += lenFn(blockContent)
			if blocksLen > chunkSize {
				flushWithBlockSplit()
			}
		}
	}

	if blocksLen > chunkSize {
		flushWithBlockSplit()
	} else {
		flush(true)
	}
	return result
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run TestSplitSource -v`
Expected: PASS

---

### Task 4: Transformer 入口（NewSplitter + Transform）

**Files:**
- Create: `pkg/eino-ext/chunker/markdown/markdown.go`
- Test: `pkg/eino-ext/chunker/markdown/markdown_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestMarkdownChunker_ImplementsTransformer(t *testing.T) {
	var _ document.Transformer = (*markdownChunker)(nil)
}

func TestMarkdownChunker_Transform_Basic(t *testing.T) {
	splitter, err := NewSplitter(context.Background(), &Config{
		ChunkSize: 500,
		LenFunc:   func(s string) int { return len(s) },
	})
	if err != nil {
		t.Fatal(err)
	}

	source := "# Hello\n\nWorld.\n"
	docs, err := splitter.Transform(context.Background(), []*schema.Document{
		{ID: "doc1", Content: source, MetaData: map[string]any{"kind": "md"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d docs, want 1", len(docs))
	}
	if docs[0].Content != source {
		t.Fatalf("content mismatch: %q", docs[0].Content)
	}
	if docs[0].MetaData["kind"] != "md" {
		t.Fatal("original metadata not preserved")
	}
	if docs[0].MetaData[MetaHeadingH1Key] != "Hello" {
		t.Fatalf("h1 = %v", docs[0].MetaData[MetaHeadingH1Key])
	}
	if docs[0].MetaData[MetaChunkByteStartKey] != 0 {
		t.Fatalf("byte_start = %v, want 0", docs[0].MetaData[MetaChunkByteStartKey])
	}
}
```

Add imports:
```go
import (
	"context"
	"maps"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
)
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run TestMarkdownChunker -v`
Expected: FAIL

- [ ] **Step 3: Write minimal implementation**

```go
// markdown.go
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
	return &markdownChunker{
		chunkSize:   config.ChunkSize,
		lenFn:       lenFn,
		idGenerator: idGen,
	}, nil
}

type markdownChunker struct {
	chunkSize   int
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
		drafts := splitSource(doc.Content, c.chunkSize, c.lenFn)
		for i, d := range drafts {
			if d.content == "" {
				continue
			}
			meta := deepCopyMap(doc.MetaData)
			for k, v := range d.meta {
				meta[k] = v
			}
			ret = append(ret, &schema.Document{
				ID:       c.idGenerator(ctx, doc.ID, i),
				Content:  d.content,
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -run TestMarkdownChunker -v`
Expected: PASS

---

### Task 5: 完整测试与 testdata

**Files:**
- Create: `pkg/eino-ext/chunker/markdown/testdata/sample.md`
- Modify: `pkg/eino-ext/chunker/markdown/markdown_test.go`

- [ ] **Step 1: Create testdata**

`testdata/sample.md`:
```markdown
# Rust 并发

Rust 通过所有权限制数据竞争。

## Send 与 Sync

Send 和 Sync 由类型系统静态保证。

```rust
fn main() {}
```
```

- [ ] **Step 2: Add integration tests**

```go
func TestMarkdownChunker_PreservesCodeBlock(t *testing.T) {
	sourceBytes, err := os.ReadFile("testdata/sample.md")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)

	splitter, err := NewSplitter(context.Background(), &Config{ChunkSize: 500})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := splitter.Transform(context.Background(), []*schema.Document{
		{ID: "sample", Content: source},
	})
	if err != nil {
		t.Fatal(err)
	}

	foundCode := false
	for _, doc := range docs {
		if strings.Contains(doc.Content, "```rust") {
			foundCode = true
			if !strings.Contains(doc.Content, "fn main()") {
				t.Fatal("code block content broken")
			}
		}
		byteStart, _ := doc.MetaData[MetaChunkByteStartKey].(int)
		byteEnd, _ := doc.MetaData[MetaChunkByteEndKey].(int)
		if source[byteStart:byteEnd] != doc.Content {
			t.Fatalf("position mismatch for chunk: %q", doc.Content[:20])
		}
	}
	if !foundCode {
		t.Fatal("code block chunk not found")
	}
}

func TestNewSplitter_Validation(t *testing.T) {
	_, err := NewSplitter(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
	_, err = NewSplitter(context.Background(), &Config{ChunkSize: 0})
	if err == nil {
		t.Fatal("expected error for zero chunk size")
	}
}
```

- [ ] **Step 3: Run all tests**

Run: `go test ./pkg/eino-ext/chunker/markdown/... -v`
Expected: ALL PASS

---

## Spec Coverage Checklist

| Spec 要求 | 对应 Task |
|---|---|
| goldmark AST 解析 | Task 3 split.go |
| Heading Section 切分 | Task 3 |
| 超限块边界拆分 | Task 3 TestSplitSource_OversizedSectionByBlock |
| 单块超限整块输出 | Task 3（block 不拆，自然满足） |
| md_h1~h6 元数据 | Task 1 |
| md_chunk_byte/rune 位置 | Task 2 + Task 3 |
| Config 校验 | Task 4 |
| document.Transformer | Task 4 |
| 独立 pkg | 全部在 pkg/eino-ext/chunker/markdown |
| 不接入 ChunkTransformer | 无相关 task |

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-28-markdown-chunker.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** — 每个 Task 派一个 subagent，task 间 review
2. **Inline Execution** — 当前 session 按 Task 顺序直接实现

Which approach?
