package datatable

import "testing"

func TestValidateDataTableMarkdown(t *testing.T) {
	if err := ValidateDataTableMarkdown("| a | b |\n| --- | --- |\n| 1 | 2 |\n"); err != nil {
		t.Fatalf("expected valid markdown table, got err=%v", err)
	}
	if err := ValidateDataTableMarkdown("# title\n| a | b |\n| --- | --- |\n| 1 | 2 |"); err == nil {
		t.Fatal("expected invalid markdown table")
	}
}

func TestNormalizeDataTableMarkdown(t *testing.T) {
	got, err := NormalizeDataTableMarkdown("```\n| a | b |\n| --- | --- |\n| 1 | 2 |\n```")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}
	want := "| a | b |\n| --- | --- |\n| 1 | 2 |\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
