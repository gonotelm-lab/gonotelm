package deptest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// DocNode 是文档树中的节点，代表一个章节
type testMarkdownDocNode struct {
	Title    string                 `json:"title,omitempty"`    // 标题文本（根节点为空）
	Level    int                    `json:"level"`              // 树层级，0 为文档最深标题层（叶子层），向上递增
	Content  []string               `json:"content,omitempty"`  // 该节下聚合后的文本内容
	Children []*testMarkdownDocNode `json:"children,omitempty"` // 子章节
}

func TestParseMarkdownDocNode(t *testing.T) {
	extractSegmentsText := func(segments *text.Segments, source []byte) string {
		if segments == nil || segments.Len() == 0 {
			return ""
		}
		var sb strings.Builder
		for i := 0; i < segments.Len(); i++ {
			segment := segments.At(i)
			sb.Write(segment.Value(source))
		}
		return strings.TrimSpace(sb.String())
	}

	extractInlineText := func(n ast.Node, source []byte) string {
		var sb strings.Builder
		_ = ast.Walk(n, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
			if !entering {
				return ast.WalkContinue, nil
			}
			switch v := child.(type) {
			case *ast.Text:
				sb.Write(v.Value(source))
				if v.SoftLineBreak() || v.HardLineBreak() {
					sb.WriteByte('\n')
				}
			case *ast.String:
				sb.Write(v.Value)
			}
			return ast.WalkContinue, nil
		})
		return strings.TrimSpace(sb.String())
	}

	var extractBlockText func(n ast.Node, source []byte) string
	extractBlockText = func(n ast.Node, source []byte) string {
		switch node := n.(type) {
		case *ast.Paragraph:
			return extractSegmentsText(node.Lines(), source)
		case *ast.List:
			var lines []string
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				if item, ok := child.(*ast.ListItem); ok {
					itemText := extractInlineText(item, source)
					if itemText == "" {
						itemText = extractSegmentsText(item.Lines(), source)
					}
					if itemText != "" {
						lines = append(lines, "- "+itemText)
					}
				}
			}
			return strings.Join(lines, "\n")
		case *ast.Blockquote:
			var blocks []string
			for child := node.FirstChild(); child != nil; child = child.NextSibling() {
				blockText := extractBlockText(child, source)
				if blockText != "" {
					blocks = append(blocks, blockText)
				}
			}
			if len(blocks) == 0 {
				return ""
			}
			content := strings.Join(blocks, "\n")
			lines := strings.Split(content, "\n")
			for i := range lines {
				lines[i] = "> " + lines[i]
			}
			return strings.Join(lines, "\n")
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			return extractSegmentsText(node.Lines(), source)
		case *ast.HTMLBlock:
			return extractSegmentsText(node.Lines(), source)
		default:
			return ""
		}
	}

	buildDocTree := func(source []byte) (*testMarkdownDocNode, error) {
		parser := goldmark.DefaultParser()
		reader := text.NewReader(source) // 修正：使用 text.NewReader
		doc := parser.Parse(reader)

		root := &testMarkdownDocNode{Level: 0, Title: "root"}
		stack := []*testMarkdownDocNode{root}
		maxHeadingLevel := 0

		current := root

		// 遍历顶级块节点
		for n := doc.FirstChild(); n != nil; n = n.NextSibling() {
			t.Log(n.Type(), n.Kind())
			switch node := n.(type) {
			case *ast.Heading:
				if node.Level > maxHeadingLevel {
					maxHeadingLevel = node.Level
				}
				for stack[len(stack)-1].Level >= node.Level {
					stack = stack[:len(stack)-1]
				}
				parent := stack[len(stack)-1]

				newNode := &testMarkdownDocNode{
					Title: extractInlineText(node, source),
					Level: node.Level,
				}
				parent.Children = append(parent.Children, newNode)
				stack = append(stack, newNode)
				current = newNode

			default:
				text := extractBlockText(node, source)
				if text != "" {
					current.Content = append(current.Content, text)
				}
			}
		}

		// 将层级从“标题深度”转换为“最深标题层为 0，向上递增”。
		var relabelLevels func(node *testMarkdownDocNode)
		relabelLevels = func(node *testMarkdownDocNode) {
			node.Level = maxHeadingLevel - node.Level

			for _, child := range node.Children {
				relabelLevels(child)
			}
		}
		relabelLevels(root)

		return root, nil
	}
	md, err := os.ReadFile("./testdata/test.md")
	if err != nil {
		t.Fatalf("read file failed, err=%v, err=%v", err, err)
	}

	tree, err := buildDocTree(md)
	if err != nil {
		t.Fatalf("build doc tree failed, err=%v, err=%v", err, err)
	}

	data, _ := json.MarshalIndent(tree, "", "  ")
	fmt.Println(string(data))
}
