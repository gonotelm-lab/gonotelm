package xlsx

import (
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func writeSheetMarkdown(
	ctx context.Context,
	b *strings.Builder,
	f *excelize.File,
	sheet string,
	includeCharts bool,
) error {
	_ = ctx

	fmt.Fprintf(b, "## %s\n\n", sheet)

	rowsIter, err := f.Rows(sheet)
	if err != nil {
		return fmt.Errorf("xlsx rows %q: %w", sheet, err)
	}
	defer func() { _ = rowsIter.Close() }()

	var (
		rows       [][]string
		maxCols    int
		hasContent bool
	)
	for rowsIter.Next() {
		cols, err := rowsIter.Columns()
		if err != nil {
			return fmt.Errorf("xlsx columns %q: %w", sheet, err)
		}
		if len(cols) > maxCols {
			maxCols = len(cols)
		}
		for _, c := range cols {
			if strings.TrimSpace(c) != "" {
				hasContent = true
				break
			}
		}
		rows = append(rows, cols)
	}
	if err := rowsIter.Error(); err != nil {
		return fmt.Errorf("xlsx iterate %q: %w", sheet, err)
	}

	if !hasContent || maxCols == 0 {
		b.WriteString("(empty)\n")
		if err := writeCommentsSection(b, f, sheet); err != nil {
			return err
		}
		writeChartsForSheet(b, f, sheet, includeCharts)
		return nil
	}

	// Synthetic header Col1..ColN
	b.WriteByte('|')
	for i := 1; i <= maxCols; i++ {
		fmt.Fprintf(b, " Col%d |", i)
	}
	b.WriteByte('\n')
	b.WriteByte('|')
	for i := 0; i < maxCols; i++ {
		b.WriteString(" --- |")
	}
	b.WriteByte('\n')

	for _, row := range rows {
		b.WriteByte('|')
		for i := 0; i < maxCols; i++ {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			fmt.Fprintf(b, " %s |", escapeCell(cell))
		}
		b.WriteByte('\n')
	}

	if err := writeCommentsSection(b, f, sheet); err != nil {
		return err
	}
	writeChartsForSheet(b, f, sheet, includeCharts)
	return nil
}

func writeCommentsSection(b *strings.Builder, f *excelize.File, sheet string) error {
	comments, err := f.GetComments(sheet)
	if err != nil {
		// No comments part is fine for many sheets.
		return nil
	}
	if len(comments) == 0 {
		return nil
	}

	var written int
	var section strings.Builder
	for _, c := range comments {
		text := commentText(c)
		if text == "" {
			continue
		}
		fmt.Fprintf(&section, "- `%s`: %s\n", c.Cell, escapeCell(text))
		written++
	}
	if written == 0 {
		return nil
	}
	b.WriteString("\n### Comments\n\n")
	b.WriteString(section.String())
	return nil
}

func commentText(c excelize.Comment) string {
	text := strings.TrimSpace(c.Text)
	if text != "" {
		return text
	}
	if len(c.Paragraph) == 0 {
		return ""
	}
	parts := make([]string, 0, len(c.Paragraph))
	for _, p := range c.Paragraph {
		parts = append(parts, p.Text)
	}
	return strings.TrimSpace(strings.Join(parts, ""))
}

func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "|", `\|`)
	return s
}
