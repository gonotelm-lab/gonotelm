package idgen

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func parseID(t *testing.T, id string) (ms int64, idx int64) {
	t.Helper()
	parts := strings.Split(id, "-")
	if len(parts) != 2 {
		t.Fatalf("invalid id format: %q", id)
	}
	ms, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		t.Fatalf("parse ms: %v", err)
	}
	idx, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		t.Fatalf("parse idx: %v", err)
	}
	return ms, idx
}

func TestGetFormatAndSequence(t *testing.T) {
	key := t.Name()
	id0 := Get(key)
	ms0, idx0 := parseID(t, id0)
	if idx0 != 0 {
		t.Fatalf("first idx = %d, want 0", idx0)
	}

	id1 := Get(key)
	ms1, idx1 := parseID(t, id1)
	if ms1 == ms0 {
		if idx1 != 1 {
			t.Fatalf("same ms: second idx = %d, want 1", idx1)
		}
		return
	}

	// millisecond rolled over between calls; idx resets
	if idx1 != 0 {
		t.Fatalf("ms rollover: second idx = %d, want 0", idx1)
	}
}

func TestGetKeyIsolation(t *testing.T) {
	a := Get("key-a")
	b := Get("key-b")

	_, idxA := parseID(t, a)
	_, idxB := parseID(t, b)
	if idxA != 0 || idxB != 0 {
		t.Fatalf("each key should start at idx 0, got a=%d b=%d", idxA, idxB)
	}
}

func TestGetConcurrent(t *testing.T) {
	key := t.Name()
	const n = 100
	ids := make([]string, n)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			ids[i] = Get(key)
		}(i)
	}
	wg.Wait()

	seen := make(map[string]struct{}, n)
	for _, id := range ids {
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id: %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestGetManyInSameMillisecond(t *testing.T) {
	key := fmt.Sprintf("%s-burst", t.Name())
	const n = 50
	ids := make([]string, n)
	for i := range n {
		ids[i] = Get(key)
	}

	ms0, _ := parseID(t, ids[0])
	for i, id := range ids {
		ms, idx := parseID(t, id)
		if ms != ms0 {
			continue
		}
		if int64(i) != idx {
			t.Fatalf("ids[%d] idx = %d, want %d (id=%q)", i, idx, i, id)
		}
	}
}

func TestLRUEviction(t *testing.T) {
	SetMaxKeys(2)
	t.Cleanup(func() { SetMaxKeys(defaultMaxKeys) })

	Get("lru-a")
	Get("lru-b")
	Get("lru-c") // evicts lru-a

	// re-insert evicted key; should start fresh at idx 0
	id := Get("lru-a")
	_, idx := parseID(t, id)
	if idx != 0 {
		t.Fatalf("re-inserted key idx = %d, want 0", idx)
	}
}

func TestSetMaxKeysShrinks(t *testing.T) {
	prefix := t.Name()
	SetMaxKeys(4)
	t.Cleanup(func() { SetMaxKeys(defaultMaxKeys) })

	for i := range 4 {
		Get(fmt.Sprintf("%s-%d", prefix, i))
	}

	SetMaxKeys(2)

	mu.Lock()
	size := cache.len()
	mu.Unlock()

	if size != 2 {
		t.Fatalf("cache len = %d, want 2", size)
	}
}
