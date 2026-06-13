package markdown

import (
	"strings"
	"testing"
)

func TestTableBuilder(t *testing.T) {
	builder := NewTableBuilder([]string{"chunk_id", "content", "score"})
	builder.AddRow([]string{"1", "line1\nline2", "0.95"})
	builder.AddRow([]string{"2", "has|pipe", "0.90"})
	builder.AddRow([]string{"3", "a\r\nb\nc", "0.85"})
	table := builder.Build()

	if strings.Contains(table, "\nline2") {
		t.Error("raw newline leaked into table")
	}
	if !strings.Contains(table, "line1<br>line2") {
		t.Error("newline not replaced with <br>")
	}
	if !strings.Contains(table, `has\|pipe`) {
		t.Error("pipe not escaped")
	}
	if !strings.Contains(table, "a<br>b<br>c") {
		t.Error("mixed line endings not handled")
	}
}

func TestTableBuilder_DynamicColumns(t *testing.T) {
	builder := NewTableBuilder([]string{"a", "b"})
	builder.AddRow([]string{"1", "2"})
	table := builder.Build()

	lines := strings.Split(strings.TrimSpace(table), "\n")
	if lines[1] != "|---|---|" {
		t.Errorf("separator = %q, want |---|---|", lines[1])
	}
}

func TestTableBuilder_SeeExample(t *testing.T) {
	builder := NewTableBuilder([]string{"chunk_id", "content", "score"})
	builder.AddRow([]string{"1", "line1\nline2", "0.95"})
	builder.AddRow([]string{"2", "has|pipe", "0.90"})
	builder.AddRow([]string{"3", "a\r\nb\nc", "0.85"})
	table := builder.Build()

	println(table)
}