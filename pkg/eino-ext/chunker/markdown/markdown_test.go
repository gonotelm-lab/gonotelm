package markdown

import (
	"context"
	"os"
	"unicode/utf8"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/document"
	"github.com/cloudwego/eino/schema"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

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
	got := string(source[br.start:br.end])
	if got != "# Title\n" {
		br, ok = extractBlockByteRange(heading, source, len(source))
		if !ok {
			t.Fatal("expected top level range ok")
		}
		got = string(source[br.start:br.end])
	}
	if got != "# Title\n" {
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

func TestSplitSource_HeadingSections(t *testing.T) {
	source := "# Rust 基础\n\n所有权系统。\n\n## 借用\n\n借用规则。\n"
	chunks := splitSource(source, 500, 0, func(s string) int { return len(s) })

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
	chunks := splitSource(source, 350, 0, func(s string) int { return len(s) })

	if len(chunks) < 2 {
		t.Fatalf("expected >= 2 chunks, got %d", len(chunks))
	}

	for _, chunk := range chunks {
		if strings.Contains(chunk.content, "A") && strings.Contains(chunk.content, "B") {
			t.Fatalf("chunk should not merge both oversized paragraphs: %q", chunk.content[:40])
		}
	}
}

func TestSplitSource_NoHeadingPreamble(t *testing.T) {
	source := "前言段落。\n\n# Title\n\n正文。\n"
	chunks := splitSource(source, 500, 0, func(s string) int { return len(s) })

	if len(chunks) != 2 {
		t.Fatalf("got %d chunks, want 2", len(chunks))
	}
	if chunks[0].content != "前言段落。\n" {
		t.Fatalf("preamble content = %q", chunks[0].content)
	}
	if _, ok := chunks[0].meta[MetaHeadingH1Key]; ok {
		t.Fatal("preamble should not have h1 metadata")
	}
}

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
			t.Fatalf("position mismatch for chunk: %q", doc.Content[:min(20, len(doc.Content))])
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

func TestSplitSource_PositionMetadata(t *testing.T) {
	source := "# Title\n\nParagraph.\n"
	chunks := splitSource(source, 500, 0, func(s string) int { return len(s) })
	if len(chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(chunks))
	}

	byteStart, _ := chunks[0].meta[MetaChunkByteStartKey].(int)
	byteEnd, _ := chunks[0].meta[MetaChunkByteEndKey].(int)
	if source[byteStart:byteEnd] != chunks[0].content {
		t.Fatal("byte range does not match content")
	}
}

func TestSplitSample2(t *testing.T) {
	sourceBytes, err := os.ReadFile("testdata/sample2.md")
	if err != nil {
		t.Fatal(err)
	}
	source := string(sourceBytes)

	splitter, err := NewSplitter(t.Context(), &Config{
		ChunkSize: 120,
		LenFunc:   func(s string) int { return utf8.RuneCountInString(s) },
	})
	if err != nil {
		t.Fatal(err)
	}

	docs, err := splitter.Transform(t.Context(), []*schema.Document{
		{ID: "sample2", Content: source},
	})
	if err != nil {
		t.Fatal(err)
	}

	for _, doc := range docs {
		println(doc.Content)
		println(strings.Repeat("-", 50))
	}
}