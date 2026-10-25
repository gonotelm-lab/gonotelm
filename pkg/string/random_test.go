package string

import (
	"testing"
)

func TestRandom(t *testing.T) {
	for _, n := range []int{1, 8, 11, 32, 128} {
		got, err := Random(n)
		if err != nil {
			t.Fatalf("Random(%d) failed: %v", n, err)
		}
		if len(got) != n {
			t.Fatalf("Random(%d) returns %d chars, want %d", n, len(got), n)
		}
		for _, c := range got {
			if !isURLSafeBase64(c) {
				t.Fatalf("Random(%d) returns non-base64 char %q", n, c)
			}
		}
	}
}

func TestRandomNonPositive(t *testing.T) {
	for _, n := range []int{0, -1} {
		got, err := Random(n)
		if err != nil {
			t.Fatalf("Random(%d) failed: %v", n, err)
		}
		if got != "" {
			t.Fatalf("Random(%d) = %q, want empty", n, got)
		}
	}
}

func TestRandomVaries(t *testing.T) {
	a, err := Random(16)
	if err != nil {
		t.Fatalf("Random failed: %v", err)
	}
	b, err := Random(16)
	if err != nil {
		t.Fatalf("Random failed: %v", err)
	}
	if a == b {
		t.Fatalf("two Random(16) calls returned the same value %q", a)
	}
}

func isURLSafeBase64(c rune) bool {
	return c >= 'a' && c <= 'z' ||
		c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' ||
		c == '-' || c == '_'
}
