package pipeline

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestData_SetAndGet(t *testing.T) {
	d := NewData()
	now := time.Now()

	d.Set("str", "hello")
	d.Set("int", 42)
	d.Set("int64", int64(42))
	d.Set("float", 3.14)
	d.Set("bool", true)
	d.Set("time", now)

	if got := d.Get("str"); got != "hello" {
		t.Fatalf("Get(str) = %v, want %q", got, "hello")
	}
	if got := d.GetString("str"); got != "hello" {
		t.Fatalf("GetString(str) = %q, want %q", got, "hello")
	}
	if got := d.GetInt("int"); got != 42 {
		t.Fatalf("GetInt(int) = %d, want 42", got)
	}
	if got := d.GetInt64("int64"); got != int64(42) {
		t.Fatalf("GetInt64(int64) = %d, want 42", got)
	}
	if got := d.GetFloat64("float"); got != 3.14 {
		t.Fatalf("GetFloat64(float) = %v, want 3.14", got)
	}
	if got := d.GetBool("bool"); !got {
		t.Fatalf("GetBool(bool) = false, want true")
	}
	if got := d.GetTime("time"); !got.Equal(now) {
		t.Fatalf("GetTime(time) = %v, want %v", got, now)
	}
}

func TestData_MissingKeyReturnsZeroValue(t *testing.T) {
	d := NewData()

	if got := d.Get("missing"); got != nil {
		t.Fatalf("Get(missing) = %v, want nil", got)
	}
	if got := d.GetString("missing"); got != "" {
		t.Fatalf("GetString(missing) = %q, want empty", got)
	}
	if got := d.GetInt("missing"); got != 0 {
		t.Fatalf("GetInt(missing) = %d, want 0", got)
	}
	if got := d.GetInt64("missing"); got != 0 {
		t.Fatalf("GetInt64(missing) = %d, want 0", got)
	}
	if got := d.GetFloat64("missing"); got != 0 {
		t.Fatalf("GetFloat64(missing) = %v, want 0", got)
	}
	if got := d.GetBool("missing"); got {
		t.Fatalf("GetBool(missing) = true, want false")
	}
	if got := d.GetTime("missing"); !got.IsZero() {
		t.Fatalf("GetTime(missing) = %v, want zero time", got)
	}
}

func TestData_WrongTypeReturnsZeroValue(t *testing.T) {
	d := NewData()
	d.Set("str", "not-an-int")
	d.Set("int", 7)
	d.Set("bool", "true")

	if got := d.GetInt("str"); got != 0 {
		t.Fatalf("GetInt on a string = %d, want 0", got)
	}
	// int is not int64: typed getters require the exact stored type.
	if got := d.GetInt64("int"); got != 0 {
		t.Fatalf("GetInt64 on an int = %d, want 0", got)
	}
	if got := d.GetBool("bool"); got {
		t.Fatalf("GetBool on a string = true, want false")
	}
}

func TestData_ConcurrentAccessIsRaceFree(t *testing.T) {
	d := NewData()

	const workers = 16
	const iters = 200

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(w int) {
			defer wg.Done()
			for i := range iters {
				d.Set("shared", w*iters+i)
				_ = d.GetInt("shared")
				d.Set(fmt.Sprintf("k%d", w), i)
				_ = d.Get(fmt.Sprintf("k%d", w))
			}
		}(w)
	}
	wg.Wait()

	if _, ok := d.Get("shared").(int); !ok {
		t.Fatalf("Get(shared) = %v, want an int", d.Get("shared"))
	}
	for w := range workers {
		if got := d.GetInt(fmt.Sprintf("k%d", w)); got != iters-1 {
			t.Fatalf("GetInt(k%d) = %d, want %d", w, got, iters-1)
		}
	}
}

func TestGet_ReturnsTypedValue(t *testing.T) {
	type point struct{ X, Y int }

	d := NewData()
	d.Set("p", point{X: 1, Y: 2})
	d.Set("s", "hello")

	if got := Get[point](d, "p"); got != (point{X: 1, Y: 2}) {
		t.Fatalf("Get[point] = %+v, want {1 2}", got)
	}
	if got := Get[string](d, "s"); got != "hello" {
		t.Fatalf("Get[string] = %q, want %q", got, "hello")
	}
}

func TestGet_MissingKeyOrWrongTypeReturnsZero(t *testing.T) {
	d := NewData()
	d.Set("n", 42)

	if got := Get[string](d, "missing"); got != "" {
		t.Fatalf("Get[string](missing) = %q, want empty", got)
	}
	if got := Get[string](d, "n"); got != "" {
		t.Fatalf("Get[string] on an int = %q, want empty", got)
	}
	if got := Get[int](d, "n"); got != 42 {
		t.Fatalf("Get[int](n) = %d, want 42", got)
	}
	if got := Get[[]int](d, "missing"); got != nil {
		t.Fatalf("Get[[]int](missing) = %v, want nil", got)
	}
}
