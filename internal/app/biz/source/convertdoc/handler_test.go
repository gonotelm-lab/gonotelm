package convertdoc

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	convertdoctransformer "github.com/gonotelm-lab/gonotelm/internal/app/biz/source/convertdoc/transformer"
	. "github.com/smartystreets/goconvey/convey"
)

func testLenFn(s string) int {
	return len(s)
}

func TestChunkTransformer_Transform_LogHTML(t *testing.T) {
	Convey("ChunkTransformer HTML split", t, func() {
		transformer := convertdoctransformer.NewChunkTransformer(200, 20, testLenFn)
		htmlContent := strings.Join([]string{
			"<h1>GonoteLM 介绍</h1>",
			"<p>这是一段用于测试 HTML 分块效果的文本。我们希望看到分块后每个 chunk 的 id、metadata 和内容预览。</p>",
			"<h2>核心能力</h2>",
			"<p>" + strings.Repeat("HTML 内容分块测试。", 20) + "</p>",
			"<h2>更多信息</h2>",
			"<p>" + strings.Repeat("这里继续补充一些段落文本。", 16) + "</p>",
		}, "\n")

		input := &schema.Document{
			ID:      "html_doc",
			Content: htmlContent,
			MetaData: map[string]any{
				"source_kind": "file",
			},
		}

		logInputSummary(t, "HTML", input.Content, []string{"h1: GonoteLM 介绍", "h2: 核心能力", "h2: 更多信息"})

		chunks, err := transformer.Transform(
			context.Background(),
			[]*schema.Document{input},
			convertdoctransformer.WithChunkSplitMethod(convertdoctransformer.ChunkHtmlSplitMethod),
		)

		So(err, ShouldBeNil)
		So(len(chunks), ShouldBeGreaterThan, 0)

		hasH1 := false
		hasH2 := false
		for _, chunk := range chunks {
			So(chunk, ShouldNotBeNil)
			So(chunk.Content, ShouldNotEqual, "")
			So(chunk.MetaData, ShouldNotBeNil)
			So(chunk.MetaData["source_kind"], ShouldEqual, "file")
			if _, ok := chunk.MetaData["doc_h1"]; ok {
				hasH1 = true
			}
			if _, ok := chunk.MetaData["doc_h2"]; ok {
				hasH2 = true
			}
		}
		So(hasH1, ShouldBeTrue)
		So(hasH2, ShouldBeTrue)

		logChunks(t, "HTML", chunks)
	})
}

func TestChunkTransformer_Transform_LogMarkdown(t *testing.T) {
	Convey("ChunkTransformer Markdown split", t, func() {
		transformer := convertdoctransformer.NewChunkTransformer(200, 20, testLenFn)
		markdownContent := strings.Join([]string{
			"# 文档标题",
			"这是一段 markdown 的正文内容，用于观察 header splitter 的分块情况。",
			"",
			"## 第一部分",
			strings.Repeat("第一部分的内容。", 24),
			"",
			"## 第二部分",
			strings.Repeat("第二部分的内容。", 20),
			"",
			"### 第二部分-子节",
			strings.Repeat("子节内容。", 18),
		}, "\n")

		input := &schema.Document{
			ID:      "md_doc",
			Content: markdownContent,
			MetaData: map[string]any{
				"source_kind": "text",
			},
		}

		logInputSummary(
			t,
			"Markdown",
			input.Content,
			[]string{"# 文档标题", "## 第一部分", "## 第二部分", "### 第二部分-子节"},
		)

		chunks, err := transformer.Transform(
			context.Background(),
			[]*schema.Document{input},
			convertdoctransformer.WithChunkSplitMethod(convertdoctransformer.ChunkMarkdownSplitMethod),
		)

		So(err, ShouldBeNil)
		So(len(chunks), ShouldBeGreaterThan, 0)

		hasH1 := false
		hasH2 := false
		hasH3 := false
		for _, chunk := range chunks {
			So(chunk, ShouldNotBeNil)
			So(chunk.Content, ShouldNotEqual, "")
			So(chunk.MetaData, ShouldNotBeNil)
			So(chunk.MetaData["source_kind"], ShouldEqual, "text")
			if _, ok := chunk.MetaData["doc_h1"]; ok {
				hasH1 = true
			}
			if _, ok := chunk.MetaData["doc_h2"]; ok {
				hasH2 = true
			}
			if _, ok := chunk.MetaData["doc_h3"]; ok {
				hasH3 = true
			}
		}
		So(hasH1, ShouldBeTrue)
		So(hasH2, ShouldBeTrue)
		So(hasH3, ShouldBeTrue)

		logChunks(t, "Markdown", chunks)
	})
}

func TestChunkTransformer_Transform_LogRecursiveFallback(t *testing.T) {
	Convey("ChunkTransformer recursive fallback split", t, func() {
		transformer := convertdoctransformer.NewChunkTransformer(120, 10, testLenFn)
		input := &schema.Document{
			ID:      "fallback_doc",
			Content: strings.Repeat("没有显式设置分块方式时会走 recursive fallback。", 30),
			MetaData: map[string]any{
				"source_kind": "text",
			},
		}

		t.Log("===== Recursive Fallback Input =====")
		t.Log(preview(input.Content, 180))

		var (
			chunks []*schema.Document
			err    error
		)
		So(func() {
			chunks, err = transformer.Transform(context.Background(), []*schema.Document{input})
		}, ShouldNotPanic)
		So(err, ShouldBeNil)
		So(len(chunks), ShouldBeGreaterThan, 1)

		for _, chunk := range chunks {
			So(chunk, ShouldNotBeNil)
			So(chunk.Content, ShouldNotEqual, "")
			So(chunk.MetaData, ShouldNotBeNil)
			So(chunk.MetaData["source_kind"], ShouldEqual, "text")
		}

		logChunks(t, "RecursiveFallback", chunks)
	})
}

func logChunks(t *testing.T, scenario string, chunks []*schema.Document) {
	t.Helper()
	t.Logf("===== %s Result =====", scenario)
	t.Logf("chunk count=%d", len(chunks))

	for idx, chunk := range chunks {
		t.Logf(
			"chunk[%d] id=%s meta=%v preview=%s",
			idx,
			chunk.ID,
			chunk.MetaData,
			preview(chunk.Content, 160),
		)
	}
}

func logInputSummary(t *testing.T, scenario, content string, markers []string) {
	t.Helper()
	t.Logf("===== %s Input Summary =====", scenario)
	t.Logf("content length=%d", len([]rune(content)))
	t.Logf("marker hints=%v", markers)
	t.Logf("content preview=%s", preview(content, 220))
}

func preview(content string, maxRunes int) string {
	flat := strings.ReplaceAll(content, "\n", "\\n")
	rs := []rune(flat)
	if len(rs) <= maxRunes {
		return flat
	}
	return string(rs[:maxRunes]) + "..."
}
