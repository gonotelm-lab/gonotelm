package types

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeTitle(t *testing.T) {
	if got := NormalizeTitle("  hello  "); got != "hello" {
		t.Fatalf("trim failed: %q", got)
	}
	if got := NormalizeTitle(""); got != "" {
		t.Fatalf("empty failed: %q", got)
	}

	long := strings.Repeat("标", MaxTitleLength+10)
	got := NormalizeTitle(long)
	if utf8.RuneCountInString(got) != MaxTitleLength {
		t.Fatalf("truncate length = %d, want %d", utf8.RuneCountInString(got), MaxTitleLength)
	}
}
