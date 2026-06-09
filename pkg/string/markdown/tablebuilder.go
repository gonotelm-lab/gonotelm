package markdown

import "strings"

type TableBuilder struct {
	header []string
	rows   [][]string
}

func NewTableBuilder(header []string) *TableBuilder {
	return &TableBuilder{
		header: header,
		rows:   make([][]string, 0),
	}
}

// AddRow adds a row to the table. The row must be the same length as the header.
func (b *TableBuilder) AddRow(row []string) {
	if len(row) != len(b.header) {
		return
	}

	b.rows = append(b.rows, row)
}

func (b *TableBuilder) Build() string {
	var builder strings.Builder
	builder.Grow(512)

	builder.WriteString("|")
	builder.WriteString(strings.Join(b.header, "|"))
	builder.WriteString("|\n|")
	for i := range b.header {
		if i > 0 {
			builder.WriteByte('|')
		}
		builder.WriteString("---")
	}
	builder.WriteString("|\n")

	for _, row := range b.rows {
		builder.WriteByte('|')
		for i, cell := range row {
			if i > 0 {
				builder.WriteByte('|')
			}
			builder.WriteString(escapeCell(cell))
		}
		builder.WriteString("|\n")
	}

	return builder.String()
}

var cellReplacer = strings.NewReplacer(
	"\r\n", "<br>",
	"\n", "<br>",
	"\r", "<br>",
	"|", "\\|",
)

func escapeCell(s string) string {
	return cellReplacer.Replace(s)
}
