package indices

import (
	"strconv"
	"strings"

	"github.com/yuin/goldmark/ast"
	goldtext "github.com/yuin/goldmark/text"
)

func joinMarkdownSegmentsText(segments *goldtext.Segments, source []byte) string {
	if segments == nil || segments.Len() == 0 {
		return ""
	}

	var sb strings.Builder
	for i := 0; i < segments.Len(); i++ {
		segment := segments.At(i)
		sb.Write(segment.Value(source))
	}

	return sb.String()
}

func extractMarkdownSegmentsTrimmedText(segments *goldtext.Segments, source []byte) string {
	return strings.TrimSpace(joinMarkdownSegmentsText(segments, source))
}

func extractMarkdownSegmentsRawText(segments *goldtext.Segments, source []byte) string {
	return joinMarkdownSegmentsText(segments, source)
}

func extractMarkdownInlineText(node ast.Node, source []byte) string {
	if node == nil {
		return ""
	}

	var sb strings.Builder
	_ = ast.Walk(node, func(child ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}

		switch n := child.(type) {
		case *ast.Text:
			sb.Write(n.Value(source))
			if n.SoftLineBreak() || n.HardLineBreak() {
				sb.WriteByte('\n')
			}
		case *ast.String:
			sb.Write(n.Value)
		}

		return ast.WalkContinue, nil
	})

	return strings.TrimSpace(sb.String())
}

func extractMarkdownCodeTextAsFence(content string, lang string) string {
	code := content
	if !strings.HasSuffix(code, "\n") {
		code += "\n"
	}

	lang = strings.TrimSpace(lang)
	fence := "```"
	if lang != "" {
		fence += lang
	}

	return fence + "\n" + code + "```"
}

func extractMarkdownListItemText(item *ast.ListItem, source []byte) string {
	if item == nil {
		return ""
	}

	var blocks []string
	for itemChild := item.FirstChild(); itemChild != nil; itemChild = itemChild.NextSibling() {
		blockText := extractMarkdownBlockText(itemChild, source)
		if blockText != "" {
			blocks = append(blocks, blockText)
		}
	}
	if len(blocks) > 0 {
		return strings.TrimSpace(strings.Join(blocks, "\n"))
	}

	itemText := extractMarkdownSegmentsTrimmedText(item.Lines(), source)
	if itemText != "" {
		return itemText
	}

	return extractMarkdownInlineText(item, source)
}

func formatMarkdownListItem(prefix string, itemText string) string {
	lines := strings.Split(itemText, "\n")
	if len(lines) == 0 {
		return prefix
	}

	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(lines[0])

	if len(lines) == 1 {
		return sb.String()
	}

	indent := strings.Repeat(" ", len(prefix))
	for i := 1; i < len(lines); i++ {
		sb.WriteByte('\n')
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		sb.WriteString(indent)
		sb.WriteString(lines[i])
	}

	return sb.String()
}

func extractMarkdownListText(node *ast.List, source []byte) string {
	if node == nil {
		return ""
	}

	var (
		lines []string
		index int
		start = node.Start
	)
	if start <= 0 {
		start = 1
	}

	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		item, ok := child.(*ast.ListItem)
		if !ok {
			continue
		}

		itemText := extractMarkdownListItemText(item, source)
		if itemText == "" {
			continue
		}

		marker := node.Marker
		if marker == 0 {
			if node.IsOrdered() {
				marker = '.'
			} else {
				marker = '-'
			}
		}

		prefix := string(marker) + " "
		if node.IsOrdered() {
			prefix = strconv.Itoa(start+index) + string(marker) + " "
		}
		index++

		lines = append(lines, formatMarkdownListItem(prefix, itemText))
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractMarkdownBlockquoteText(node *ast.Blockquote, source []byte) string {
	if node == nil {
		return ""
	}

	var blocks []string
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		blockText := extractMarkdownBlockText(child, source)
		if blockText != "" {
			blocks = append(blocks, blockText)
		}
	}
	if len(blocks) == 0 {
		return ""
	}

	content := strings.Join(blocks, "\n\n")
	lines := strings.Split(content, "\n")
	for i := range lines {
		if strings.TrimSpace(lines[i]) == "" {
			lines[i] = ">"
			continue
		}
		lines[i] = "> " + lines[i]
	}

	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractMarkdownBlockText(node ast.Node, source []byte) string {
	switch n := node.(type) {
	case *ast.Paragraph, *ast.HTMLBlock:
		return extractMarkdownSegmentsTrimmedText(n.Lines(), source)
	case *ast.FencedCodeBlock:
		return extractMarkdownCodeTextAsFence(
			extractMarkdownSegmentsRawText(n.Lines(), source),
			string(n.Language(source)),
		)
	case *ast.CodeBlock:
		return extractMarkdownCodeTextAsFence(
			extractMarkdownSegmentsRawText(n.Lines(), source),
			"",
		)
	case *ast.List:
		return extractMarkdownListText(n, source)
	case *ast.Blockquote:
		return extractMarkdownBlockquoteText(n, source)
	default:
		return extractMarkdownInlineText(n, source)
	}
}
