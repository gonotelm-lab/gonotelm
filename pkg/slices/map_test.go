package slices

import (
	"maps"
	"testing"
)

func TestAsSet(t *testing.T) {
	got := AsSet([]int{1, 2, 2, 3})
	want := map[int]struct{}{
		1: {},
		2: {},
		3: {},
	}

	if !maps.Equal(got, want) {
		t.Fatalf("unexpected set, got=%v want=%v", got, want)
	}
}

func TestAsSetF(t *testing.T) {
	type user struct {
		ID   int
		Name string
	}

	got := AsSetF([]user{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 1, Name: "alice-duplicate"},
	}, func(u user) int {
		return u.ID
	})
	want := map[int]struct{}{
		1: {},
		2: {},
	}

	if !maps.Equal(got, want) {
		t.Fatalf("unexpected set by key, got=%v want=%v", got, want)
	}
}

func TestAsMap(t *testing.T) {
	got := AsMap([]string{"a", "b", "a"})
	want := map[string]string{
		"a": "a",
		"b": "b",
	}

	if !maps.Equal(got, want) {
		t.Fatalf("unexpected map, got=%v want=%v", got, want)
	}
}

func TestAsMapF(t *testing.T) {
	type user  struct {
		id int
		name string
	}

	users := []user{
		{id: 1, name: "alice"},
		{id: 2, name: "bob"},
		{id: 3, name: "charlie"},
	}
	got := AsMapF(users, func(u user) int {
		return u.id
	})
	want := map[int]user{
		1: {id: 1, name: "alice"},
		2: {id: 2, name: "bob"},
		3: {id: 3, name: "charlie"},
	}

	if !maps.Equal(got, want) {
		t.Fatalf("unexpected map, got=%v want=%v", got, want)
	}
}	

func TestAsMapF_LastValueWins(t *testing.T) {
	type user struct {
		ID   int
		Name string
	}

	got := AsMapF([]user{
		{ID: 1, Name: "alice"},
		{ID: 2, Name: "bob"},
		{ID: 1, Name: "alice-latest"},
	}, func(u user) int {
		return u.ID
	})

	if len(got) != 2 {
		t.Fatalf("unexpected map size, got=%d want=2", len(got))
	}
	if got[1].Name != "alice-latest" {
		t.Fatalf("expected latest value for duplicated key, got=%q", got[1].Name)
	}
	if got[2].Name != "bob" {
		t.Fatalf("unexpected value for key=2, got=%q", got[2].Name)
	}
}

func TestAsMapAndSet_EmptyInput(t *testing.T) {
	if len(AsSet[int](nil)) != 0 {
		t.Fatalf("expected empty set for nil input")
	}
	if len(AsMap[int](nil)) != 0 {
		t.Fatalf("expected empty map for nil input")
	}
}
