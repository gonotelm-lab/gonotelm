package markdown

import "testing"

func TestValidateExclusivePipeTable_OK(t *testing.T) {
	content := `
| Name | Value |
| --- | ---: |
| a | 1 |
| b | 2 |
`
	if err := ValidateExclusivePipeTable(content); err != nil {
		t.Fatalf("expected valid table, got err=%v", err)
	}

	normalized, err := NormalizeExclusivePipeTable("```markdown\n| a | b |\n| --- | --- |\n| 1 | 2 |\n```")
	if err != nil {
		t.Fatalf("expected fenced table to normalize, got err=%v", err)
	}
	if normalized != "| a | b |\n| --- | --- |\n| 1 | 2 |\n" {
		t.Fatalf("unexpected normalized table: %q", normalized)
	}
}

func TestValidateExclusivePipeTable_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "empty", content: "   "},
		{name: "prose", content: "hello\n| a | b |\n| --- | --- |\n| 1 | 2 |"},
		{name: "missing_separator", content: "| a | b |\n| 1 | 2 |\n| 3 | 4 |"},
		{name: "column_mismatch", content: "| a | b |\n| --- | --- |\n| 1 |"},
		{name: "blank_inside", content: "| a | b |\n| --- | --- |\n\n| 1 | 2 |"},
		{name: "inner_fence", content: "| a | b |\n| --- | --- |\n| ``` | 2 |"},
		{name: "only_header", content: "| a | b |\n| --- | --- |"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateExclusivePipeTable(tc.content); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
