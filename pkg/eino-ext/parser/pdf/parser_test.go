package pdf

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPDFParser_Parse(t *testing.T) {
	path := "../../../../gotest/test.pdf"
	content, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	parser := NewPDFParser(&Config{})
	data, err := parser.Parse(t.Context(), content)
	if err != nil {
		t.Fatal(err)
	}

	for _, doc := range data {
		fmt.Println(doc.Content)
	}
}

func TestPDFParser_Parse2(t *testing.T) {
	path := "../../../../gotest/test2.pdf"
	content, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}

	parser := NewPDFParser(&Config{})
	data, err := parser.Parse(t.Context(), content)
	if err != nil {
		t.Fatal(err)
	}

	for _, doc := range data {
		fmt.Println(doc.Content)
	}
}

func TestToMarkdownTable_WithSeparator(t *testing.T) {
	markdown := toMarkdownTable([][]string{
		{"Name", "Score"},
		{"Alice", "100"},
		{"Bob", "9"},
	}, true)

	expected := "|Name |Score|\n|-----|-----|\n|Alice|100  |\n|Bob  |9    |"
	if markdown != expected {
		t.Fatalf("unexpected markdown table:\n%s", markdown)
	}
}

func TestMergeStructuredCharsToWords(t *testing.T) {
	merged := mergeStructuredCharsToWords([]pdfRect{
		{text: "p", left: 10, right: 12, top: 100, bottom: 98},
		{text: "r", left: 12.5, right: 14.5, top: 100, bottom: 98},
		{text: "o", left: 15, right: 17, top: 100, bottom: 98},
		{text: "t", left: 17.5, right: 19.5, top: 100, bottom: 98},
		{text: "o", left: 20, right: 22, top: 100, bottom: 98},
		{text: "c", left: 22.5, right: 24.5, top: 100, bottom: 98},
		{text: "x", left: 40, right: 42, top: 100, bottom: 98},
	})

	if len(merged) != 2 {
		t.Fatalf("expected 2 merged words, got %d", len(merged))
	}
	if merged[0].text != "protoc" || merged[1].text != "x" {
		t.Fatalf("unexpected merge result: %#v", merged)
	}
}

func TestShouldUseFormMarkdown(t *testing.T) {
	formLike := strings.Join([]string{
		"|A|B|C|",
		"|-|-|-|",
		"|1|2|3|",
		"|4|5|6|",
		"说明文本",
	}, "\n")
	if !shouldUseFormMarkdown(formLike) {
		t.Fatal("expected table-dense markdown to use form extraction")
	}

	proseLike := strings.Join([]string{
		"这是普通正文第一段",
		"这是普通正文第二段",
		"|A|B|",
		"|1|2|",
	}, "\n")
	if shouldUseFormMarkdown(proseLike) {
		t.Fatal("expected prose-dense markdown to fallback plain extraction")
	}

	noisyForm := strings.Join([]string{
		"|A|B|C|",
		"|-|-|-|",
		"|1|2|3|",
		"b i n d P o r t : 6 4 4 3",
		"c o n f i g",
	}, "\n")
	if shouldUseFormMarkdown(noisyForm) {
		t.Fatal("expected noisy single-rune content to fallback plain extraction")
	}

	driftColumns := strings.Join([]string{
		"|A|B|C|",
		"|1|2|3|",
		"|x|||||||||y|",
		"|m|n|",
	}, "\n")
	if shouldUseFormMarkdown(driftColumns) {
		t.Fatal("expected unstable table columns to fallback plain extraction")
	}
}

func TestCodeLikeTableToCodeBlock(t *testing.T) {
	table := [][]string{
		{"1", "f u n c", "main ( )", "{"},
		{"2", "r e t u r n", "n i l"},
		{"3", "}"},
	}
	if !isCodeLikeTableData(table) {
		t.Fatal("expected code-like table to be detected")
	}

	code := renderCodeBlockFromTableData(table)
	if !strings.Contains(code, "func main() {") {
		t.Fatalf("unexpected code block content: %s", code)
	}
	if !strings.Contains(code, "return") || !strings.Contains(code, "nil") {
		t.Fatalf("unexpected code block content: %s", code)
	}
}

func TestRenderStructuredLinesMarkdown_Heading(t *testing.T) {
	lines := []pdfTextLine{
		{
			yKey:     100,
			words:    []pdfRect{{text: "插件开发教程"}},
			fontSize: 20,
			fontName: "SourceHanSans-Bold",
		},
		{
			yKey: 95,
			words: []pdfRect{
				{text: "这是一段较长的正文内容，用于让正文字体成为主导。"},
			},
			fontSize: 12,
			fontName: "SourceHanSans-Regular",
		},
	}

	markdown := renderStructuredLinesMarkdown(lines)
	if !strings.Contains(markdown, "# 插件开发教程") {
		t.Fatalf("expected heading markdown, got: %s", markdown)
	}
	if !hasMarkdownHeading(markdown) {
		t.Fatalf("expected markdown heading detection, got: %s", markdown)
	}
}

func TestDetectHeadingLevel_ListLikeGuard(t *testing.T) {
	line := pdfTextLine{
		yKey:     100,
		words:    []pdfRect{{text: "一 Master"}},
		fontSize: 20,
		fontName: "SourceHanSans-Bold",
	}
	level := detectHeadingLevel(line, 12, "一 Master")
	if level != 0 {
		t.Fatalf("expected list-like line not to be heading, got level=%d", level)
	}

	englishLine := pdfTextLine{
		yKey:     100,
		words:    []pdfRect{{text: "1) Setup Environment"}},
		fontSize: 20,
		fontName: "SourceSans-Bold",
	}
	level = detectHeadingLevel(englishLine, 12, "1) Setup Environment")
	if level != 0 {
		t.Fatalf("expected english list-like line not to be heading, got level=%d", level)
	}
}

func TestEscapeProbableCodeCommentHeadings(t *testing.T) {
	content := strings.Join([]string{
		"$ kubeadm init",
		"# 创建 Master 节点",
		"$ kubeadm join ...",
		"",
		"## 正常标题",
	}, "\n")

	escaped := escapeProbableCodeCommentHeadings(content)
	if !strings.Contains(escaped, `\# 创建 Master 节点`) {
		t.Fatalf("expected code comment heading to be escaped, got: %s", escaped)
	}
	if !strings.Contains(escaped, "## 正常标题") {
		t.Fatalf("expected normal markdown heading to remain, got: %s", escaped)
	}
}

func TestNormalizeUnknownIcons(t *testing.T) {
	input := "�� 1. 标题\n\ue0a0\ue0a1 提示"
	normalized := normalizePDFPlainText(input)
	if !strings.Contains(normalized, "• 1. 标题") {
		t.Fatalf("expected replacement chars to normalize as bullet, got: %s", normalized)
	}
	if !strings.Contains(normalized, "• 提示") {
		t.Fatalf("expected private-use chars to normalize as bullet, got: %s", normalized)
	}
}
