package string

import (
	"testing"
)

func TestLast(t *testing.T) {
	if LastRune("1234567", 3) != "567" {
		t.Fatal("should be 567")
	}

	if LastRune("1234567", 7) != "1234567" {
		t.Fatal("should be 1234567")
	}

	if LastRune("1234567", 0) != "" {
		t.Fatal("should be empty")
	}

	if LastRune("12345", -1) != "" {
		t.Fatal("should be empty")
	}

	if LastRune("12345", 10) != "12345" {
		t.Fatal("should be 12345")
	}
}
