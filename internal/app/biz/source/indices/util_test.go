package indices

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	goldtext "github.com/yuin/goldmark/text"
)

func parseFirstMarkdownBlock(markdown string) ast.Node {
	source := []byte(markdown)
	doc := goldmark.DefaultParser().Parse(goldtext.NewReader(source))
	return doc.FirstChild()
}

func TestExtractMarkdownSegmentsTextVariants(t *testing.T) {
	Convey("segments 文本提取支持 trimmed/raw 语义", t, func() {
		source := []byte("  hello markdown  \n")
		segments := goldtext.NewSegments()
		segments.Append(goldtext.NewSegment(0, len(source)))

		trimmed := extractMarkdownSegmentsTrimmedText(segments, source)
		raw := extractMarkdownSegmentsRawText(segments, source)

		So(trimmed, ShouldEqual, "hello markdown")
		So(raw, ShouldEqual, "  hello markdown  \n")
	})
}

func TestExtractMarkdownBlockText(t *testing.T) {
	tests := []struct {
		name     string
		markdown string
		want     string
	}{
		{
			name: "fenced code keeps language",
			markdown: "```go\n" +
				"func main() {}\n" +
				"```\n",
			want: "```go\nfunc main() {}\n```",
		},
		{
			name: "indented code normalized to fenced",
			markdown: "    line1\n" +
				"    line2\n",
			want: "```\nline1\nline2\n```",
		},
		{
			name: "ordered list keeps start and marker",
			markdown: "3) one\n" +
				"4) two\n",
			want: "3) one\n4) two",
		},
		{
			name: "unordered list keeps marker",
			markdown: "+ one\n" +
				"+ two\n",
			want: "+ one\n+ two",
		},
		{
			name: "multiline list item keeps continuation indentation",
			markdown: "1. first line\n" +
				"   second line\n",
			want: "1. first line\n   second line",
		},
		{
			name: "blockquote keeps empty line",
			markdown: "> quote a\n" +
				">\n" +
				"> quote b\n",
			want: "> quote a\n>\n> quote b",
		},
		{
			name: "nested list keeps structure",
			markdown: "- parent\n" +
				"  - child\n",
			want: "- parent\n  - child",
		},
		{
			name: "nested blockquote keeps structure",
			markdown: "> outer\n" +
				">\n" +
				"> > inner\n",
			want: "> outer\n>\n> > inner",
		},
	}

	Convey("extractMarkdownBlockText 尽量保留原始 markdown 结构", t, func() {
		for _, tt := range tests {
			tt := tt
			Convey(tt.name, func() {
				node := parseFirstMarkdownBlock(tt.markdown)
				So(node, ShouldNotBeNil)
				got := extractMarkdownBlockText(node, []byte(tt.markdown))
				So(got, ShouldEqual, tt.want)
			})
		}
	})
}

func TestExtractMarkdownBlockTextNoPanic(t *testing.T) {
	Convey("extractMarkdownBlockText 在边界输入下不应 panic", t, func() {
		orderedList := ast.NewList('.')
		orderedList.Start = 1

		cases := []struct {
			name   string
			node   ast.Node
			source []byte
		}{
			{
				name:   "nil node",
				node:   nil,
				source: nil,
			},
			{
				name:   "empty paragraph node",
				node:   ast.NewParagraph(),
				source: nil,
			},
			{
				name:   "empty list node",
				node:   ast.NewList('-'),
				source: nil,
			},
			{
				name:   "empty ordered list node",
				node:   orderedList,
				source: nil,
			},
			{
				name:   "empty blockquote node",
				node:   ast.NewBlockquote(),
				source: nil,
			},
			{
				name:   "list item node as input",
				node:   ast.NewListItem(0),
				source: nil,
			},
			{
				name:   "unclosed fenced code from parser",
				node:   parseFirstMarkdownBlock("```go\nfmt.Println(1)\n"),
				source: []byte("```go\nfmt.Println(1)\n"),
			},
		}

		for _, tc := range cases {
			tc := tc
			Convey(tc.name, func() {
				So(func() {
					_ = extractMarkdownBlockText(tc.node, tc.source)
				}, ShouldNotPanic)
			})
		}
	})
}
