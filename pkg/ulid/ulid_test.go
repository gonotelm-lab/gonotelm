package ulid

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestULIDCompare(t *testing.T) {
	u1 := New()
	u2 := New()
	if u1 == u2 {
		t.Errorf("u1 == u2, want false")
	}

	if !u1.LessThan(u2) {
		t.Errorf("u1.LessThan(u2) == false, want true")
	}

	u3 := u1.Duplicate()
	if u3 != u1 {
		t.Errorf("u3 != u1, want true")
	}

	if u1.Compare(u2) == 0 {
		t.Errorf("u1.Compare(u2) == 0, want not 0")
	}

	if u1.Compare(EmptyULID()) != 1 {
		t.Errorf("u1.Compare(EmptyULID()) != 1, want 1")
	}
}

func TestULIDParse(t *testing.T) {
	id := New()
	t.Log(id.String())

	parsed, err := ParseString(id.String())
	if err != nil {
		t.Errorf("ParseString failed: %v", err)
	}
	t.Log(parsed.String())

	if id.Compare(parsed) != 0 {
		t.Errorf("id != parsed")
	}
}

func TestULIDMonotonic(t *testing.T) {
	u1 := New()
	u2 := New()
	t.Log(u1.String())
	t.Log(u2.String())
	t.Log(u2.Compare(u1))

	ulids := make([]ULID, 0, 1000)
	for range 1000 {
		ulids = append(ulids, New())
	}

	for i := 0; i < 999; i++ {
		if i%50 == 0 {
			t.Logf("%d: %d", i, ulids[i].UnixMilli())
		}
		if ulids[i].Compare(ulids[i+1]) >= 0 {
			t.Errorf("ulid[%d] >= ulid[%d], want less than", i, i+1)
		}
	}
}

func TestULIDTime(t *testing.T) {
	id := New()
	t.Logf("Time: %v", id.Timestamp())
	t.Logf("UnixMilli: %d", id.UnixMilli())
	t.Logf("Time from unixmilli: %v", time.UnixMilli(id.UnixMilli()))

	if id.UnixMilli() != id.Timestamp().UnixMilli() {
		t.Errorf("UnixMilli mismatch")
	}
}

func TestULIDEmpty(t *testing.T) {
	id := EmptyULID()
	t.Log(id.String())
	if !id.IsZero() {
		t.Errorf("EmptyULID().IsZero() == false, want true")
	}

	newID := New()
	if newID.Compare(EmptyULID()) != 1 {
		t.Errorf("New().Compare(EmptyULID()) != 1")
	}
}

func TestULIDFromBytes(t *testing.T) {
	id := New()
	bytes := id.Bytes()

	parsed, err := FromBytes(bytes)
	if err != nil {
		t.Errorf("FromBytes failed: %v", err)
	}
	if id.Compare(parsed) != 0 {
		t.Errorf("parsed != original")
	}
}

func TestULIDStringLowercase(t *testing.T) {
	for range 1000 {
		id := New()
		s := id.String()
		if len(s) != 26 {
			t.Errorf("len(%s) = %d, want 26", s, len(s))
		}
		for _, c := range s {
			if c >= 'A' && c <= 'Z' {
				t.Errorf("String() contains uppercase char %c in %s", c, s)
			}
		}
		if strings.ToLower(id.ULID.String()) != s {
			t.Errorf("String() = %s, want %s", s, strings.ToLower(id.ULID.String()))
		}
	}
}

func TestULIDString(t *testing.T) {
	id := New()
	t.Log(id.String())
}

func TestConcurrencySafe(t *testing.T) {
	var wg sync.WaitGroup
	ids := make([]ULID, 1000)
	for i := range 100 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for range 1000 {
				id := New()
				ids[(n*10+int(id.ULID[0]))%1000] = id
				_ = id.String()
				_ = id.Timestamp()
				_ = id.UnixMilli()
				_ = id.Bytes()
				_ = id.Compare(EmptyULID())
				_, _ = id.MarshalText()
				parsed, err := ParseString(id.String())
				if err != nil {
					t.Errorf("ParseString failed: %v", err)
					return
				}
				if parsed.Compare(id) != 0 {
					t.Errorf("parsed != id")
					return
				}
				_, _ = id.Value()
			}
		}(i)
	}
	wg.Wait()
}

func TestConcurrentSameMillisecond(t *testing.T) {
	var wg sync.WaitGroup
	ch := make(chan ULID, 10000)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				ch <- New()
			}
		}()
	}
	wg.Wait()
	close(ch)

	// Generation is serialized by the global monotonic entropy source,
	// so all ULIDs must be unique. Receive order is not guaranteed to
	// match generation order.
	seen := make(map[ULID]struct{}, 8000)
	for id := range ch {
		if _, dup := seen[id]; dup {
			t.Errorf("duplicate ulid: %s", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != 8000 {
		t.Errorf("got %d unique ulids, want 8000", len(seen))
	}
}

func TestULIDValueScanRoundTrip(t *testing.T) {
	id := New()
	v, err := id.Value()
	if err != nil {
		t.Fatal(err)
	}
	b, ok := v.([]byte)
	if !ok || len(b) != 16 {
		t.Fatalf("Value() want []byte len 16, got %T %#v", v, v)
	}

	var got ULID
	if err := got.Scan(b); err != nil {
		t.Fatal(err)
	}
	if !got.EqualsTo(id) {
		t.Fatalf("scan mismatch: got %s want %s", got, id)
	}
}

func TestULIDMarshalTextLowercase(t *testing.T) {
	id := MustParseString("01hf7yat00vtpvxvyaztxbw001")
	text, err := id.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(text) != id.String() {
		t.Fatalf("MarshalText=%q String=%q", text, id.String())
	}
	if strings.ToLower(string(text)) != string(text) {
		t.Fatalf("MarshalText not lowercase: %q", text)
	}

	var got ULID
	if err := got.UnmarshalText(text); err != nil {
		t.Fatal(err)
	}
	if !got.EqualsTo(id) {
		t.Fatalf("UnmarshalText mismatch")
	}
}

func TestULIDScanBinary(t *testing.T) {
	id := New()

	var got ULID
	if err := got.Scan(id.Bytes()); err != nil {
		t.Fatalf("Scan []byte: %v", err)
	}
	if !got.EqualsTo(id) {
		t.Fatalf("got %s want %s", got, id)
	}
}
