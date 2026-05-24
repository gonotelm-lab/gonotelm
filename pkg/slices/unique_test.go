package slices

import "testing"

func TestUniqueCount(t *testing.T) {
	got := UniqueCount([]int{1, 2, 2, 3})
	want := 3

	if got != want {
		t.Fatalf("unexpected unique count, got=%d want=%d", got, want)
	}
}
