package rg

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFindAllMatches_StringInput(t *testing.T) {
	matches, err := FindAllMatchesString("[warning]...\n[ERROR]...", &Params{
		Pattern:    "error|warning",
		IgnoreCase: true,
	})
	if err != nil {
		t.Fatalf("FindAllMatchesString() error = %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want %d", len(matches), 2)
	}
	if matches[0].Text != "warning" || matches[1].Text != "ERROR" {
		t.Fatalf("matches text = %#v, want [warning ERROR]", matches)
	}
}

func TestFindAllMatches_BytesInput_HeadLimit(t *testing.T) {
	matches, err := FindAllMatches([]byte("ab xx ab yy ab"), &Params{
		Pattern:   "ab",
		HeadLimit: 2,
	})
	if err != nil {
		t.Fatalf("FindAllMatches() error = %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("len(matches) = %d, want %d", len(matches), 2)
	}
	if matches[0].Start != 0 || matches[1].Start != 6 {
		t.Fatalf("matches start = [%d %d], want [0 6]", matches[0].Start, matches[1].Start)
	}
}

func TestGrep_ContentMode_DefaultMatches(t *testing.T) {
	got, err := GrepString("id=101 id=202", &Params{
		Pattern: `\d+`,
	})
	if err != nil {
		t.Fatalf("GrepString() error = %v", err)
	}

	if got != "101\n202" {
		t.Fatalf("GrepString() = %q, want %q", got, "101\n202")
	}
}

func TestGrep_ContentMode_Context_LineNumber_HeadLimit(t *testing.T) {
	got, err := GrepString("line-1\nmatch-alpha\nline-3\nmatch-beta\nline-5", &Params{
		Pattern:    `match-\w+`,
		OutputMode: OutputModeContent,
		Context:    1,
		LineNumber: true,
		HeadLimit:  4,
	})
	if err != nil {
		t.Fatalf("GrepString() error = %v", err)
	}

	want := strings.Join([]string{
		"1:line-1",
		"2:match-alpha",
		"3:line-3",
		"4:match-beta",
	}, "\n")
	if got != want {
		t.Fatalf("GrepString() = %q, want %q", got, want)
	}
}

func TestGrep_CountMode_IgnoreCase(t *testing.T) {
	got, err := GrepString("Error error ERROR", &Params{
		Pattern:    "error",
		OutputMode: OutputModeCount,
		IgnoreCase: true,
	})
	if err != nil {
		t.Fatalf("GrepString() error = %v", err)
	}

	if got != "3" {
		t.Fatalf("GrepString() = %q, want %q", got, "3")
	}
}

func TestGrep_MultilineLargeFixture_Params(t *testing.T) {
	fixture := readTestFixture(t, "large_multiline_sample.txt")
	pattern := `## Segment Catalog.*?Duration: 20 minutes`

	got, err := GrepString(fixture, &Params{
		Pattern: pattern,
	})
	if err != nil {
		t.Fatalf("GrepString() error = %v", err)
	}
	if got != "" {
		t.Fatalf("GrepString() = %q, want empty without multiline", got)
	}

	got, err = GrepString(fixture, &Params{
		Pattern:   pattern,
		Multiline: true,
	})
	if err != nil {
		t.Fatalf("GrepString() error = %v", err)
	}
	if !strings.Contains(got, "## Segment Catalog") {
		t.Fatalf("GrepString() = %q, expect multiline match content", got)
	}

	got, err = GrepString(fixture, &Params{
		Pattern:       pattern,
		OutputMode:    OutputModeContent,
		BeforeContext: 1,
		AfterContext:  1,
		LineNumber:    true,
		Multiline:     true,
		HeadLimit:     4,
	})
	if err != nil {
		t.Fatalf("GrepString() error = %v", err)
	}

	want := strings.Join([]string{
		"20:",
		"21:## Segment Catalog",
		"22:",
		"23:- Segment: Ownership Basics",
	}, "\n")
	if got != want {
		t.Fatalf("GrepString() = %q, want %q", got, want)
	}

	matches, err := FindAllMatchesString(fixture, &Params{
		Pattern:   pattern,
		Multiline: true,
	})
	if err != nil {
		t.Fatalf("FindAllMatchesString() error = %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("FindAllMatchesString() expect at least one match")
	}
}

func TestParams_WithoutTextContentFields(t *testing.T) {
	if _, ok := reflect.TypeOf(Params{}).FieldByName("Text"); ok {
		t.Fatal("Params should not contain Text field")
	}
	if _, ok := reflect.TypeOf(Params{}).FieldByName("Content"); ok {
		t.Fatal("Params should not contain Content field")
	}
}

func TestGrep_RequirePattern(t *testing.T) {
	_, err := GrepString("x", &Params{})
	if err == nil {
		t.Fatal("GrepString() expected error for missing pattern")
	}
}

func TestGrep_InvalidPattern(t *testing.T) {
	_, err := GrepString("content", &Params{
		Pattern: "(",
	})
	if err == nil {
		t.Fatal("GrepString() expected error for invalid pattern")
	}
}

func TestGrep_OutputMode_FileWithMatchesRemoved(t *testing.T) {
	cases := []OutputMode{
		OutputMode("file_with_matches"),
		OutputMode("files_with_matches"),
	}

	for _, mode := range cases {
		_, err := GrepString("hello", &Params{
			Pattern:    "h",
			OutputMode: mode,
		})
		if err == nil {
			t.Fatalf("GrepString() with mode %q expected error", mode)
		}
		if !strings.Contains(err.Error(), "unsupported output_mode") {
			t.Fatalf("GrepString() with mode %q error = %v, want unsupported message", mode, err)
		}
	}
}

func TestGrep_DebugExamples(t *testing.T) {
	fixture := readTestFixture(t, "large_multiline_sample.txt")

	cases := []struct {
		name    string
		text    string
		content []byte
		pattern string
		params  *Params
	}{
		{
			name:    "basic_content_numbers",
			text:    "id=101 id=202 id=303",
			pattern: `\d+`,
		},
		{
			name:    "ignore_case_count",
			text:    "Error error ERROR eRrOr",
			pattern: `error`,
			params: &Params{
				OutputMode: OutputModeCount,
				IgnoreCase: true,
			},
		},
		{
			name:    "bytes_input_with_head_limit",
			content: []byte("ab xx ab yy ab zz ab"),
			pattern: `ab`,
			params: &Params{
				HeadLimit: 3,
			},
		},
		{
			name:    "multiline_markdown_block",
			text:    fixture,
			pattern: `## Segment Catalog.*?Duration: 20 minutes`,
			params: &Params{
				Multiline: true,
			},
		},
		{
			name:    "line_context_and_numbering",
			text:    fixture,
			pattern: `## Segment Catalog.*?Duration: 20 minutes`,
			params: &Params{
				OutputMode:    OutputModeContent,
				BeforeContext: 1,
				AfterContext:  1,
				LineNumber:    true,
				Multiline:     true,
				HeadLimit:     6,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.text
			if tc.content != nil {
				source = string(tc.content)
			}

			// 为每个 case 构造独立参数，避免 nil 指针和跨用例共享状态。
			params := &Params{}
			if tc.params != nil {
				*params = *tc.params
			}
			if params.Pattern == "" {
				params.Pattern = tc.pattern
			}

			t.Logf("source preview:\n%s", source)
			t.Logf(
				"pattern=%q mode=%q ignoreCase=%v multiline=%v B=%d A=%d C=%d lineNumber=%v headLimit=%d",
				params.Pattern,
				params.OutputMode,
				params.IgnoreCase,
				params.Multiline,
				params.BeforeContext,
				params.AfterContext,
				params.Context,
				params.LineNumber,
				params.HeadLimit,
			)

			var (
				out string
				err error
			)
			if tc.content != nil {
				out, err = Grep(tc.content, params)
			} else {
				out, err = GrepString(tc.text, params)
			}
			if err != nil {
				t.Fatalf("Grep() error = %v", err)
			}
			t.Logf("grep output:\n%s", out)

			var matches []Match
			if tc.content != nil {
				matches, err = FindAllMatches(tc.content, params)
			} else {
				matches, err = FindAllMatchesString(tc.text, params)
			}
			if err != nil {
				t.Fatalf("FindAllMatches() error = %v", err)
			}
			t.Logf("structured matches: %d", len(matches))
			for i, m := range matches {
				t.Logf("match[%d] start=%d end=%d text=%q", i, m.Start, m.End, m.Text)
			}
		})
	}
}

func TestFindSimple(t *testing.T) {
	text := readTestFixture(t, "large_multiline_sample.txt")
	output, err := GrepString(text, &Params{
		Pattern:    "Audience",
		Multiline:  true,
		Context:    5,
		LineNumber: true,
		OutputMode: OutputModeCount,
	})
	t.Log(err)
	t.Log(output)
}

func readTestFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%q) failed: %v", path, err)
	}
	return string(raw)
}
