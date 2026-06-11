package slices

import "testing"

func TestUniqueCount(t *testing.T) {
	got := UniqueCount([]int{1, 2, 2, 3})
	want := 3

	if got != want {
		t.Fatalf("unexpected unique count, got=%d want=%d", got, want)
	}
}

func TestUniqueyFn(t *testing.T) {
	data := []struct{
		ID int
		Name string
	}{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 2, Name: "bob"},
		{ID: 3, Name: "charlie"},
	}

	t.Log(data)

	got := UniqueyFn(data, func(item struct{ID int; Name string}) int {
		return item.ID
	})
	t.Log(got)
}