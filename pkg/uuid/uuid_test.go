package uuid

import (
	"encoding/base64"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
)

func TestUUIDCompare(t *testing.T) {
	u1 := NewV7()
	u2 := NewV7()
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

	if u1.Compare(EmptyUUID()) != 1 {
		t.Errorf("u1.Compare(EmptyUUID()) != 1, want 1")
	}
}

func TestUUid(t *testing.T) {
	id, _ := uuid.NewV7()
	t.Log(id)

	parsed, err := ParseString(id.String())
	t.Log(err)
	t.Log(parsed)

	t.Log(MaxUUID())

	t.Log(EmptyUUID().String())
}

func TestMonotonic(t *testing.T) {
	u1 := NewV7() // smaller
	u2 := NewV7() // greater
	t.Log(u1)
	t.Log(u2)
	t.Log(u2.Compare(u1))

	uuids := make([]UUID, 0, 1000) // asc order
	for range 1000 {
		uuids = append(uuids, NewV7())
	}

	// check uuids is asc order
	for i := 0; i < 999; i++ {
		if i%50 == 0 {
			// print timestamp
			t.Logf("%d: %d", i, uuids[i].Time().UnixMilli())
		}
		if uuids[i].Compare(uuids[i+1]) >= 0 {
			t.Errorf("uuid[%d] >= uuid[%d], want less than", i, i+1)
		}
	}
}

func TestUUIDString(t *testing.T) {
	id := NewV7()
	parsed, err := ParseString(id.String())
	if err != nil {
		t.Errorf("ParseString(%s) failed, err: %v", id.String(), err)
	}
	// id should equal to parsed
	if id.Compare(parsed) != 0 {
		t.Errorf("id != parsed, want true")
	}
	t.Log(id.String())
	t.Log(parsed.String())
	t.Log(parsed.UUID.String())
}

func TestUUIDBase64(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		id := NewV7()
		t.Log(id.Base64())
		time.Sleep(time.Minute)
		id = NewV7()
		t.Log(id.Base64())
		synctest.Wait()
	})
}

func TestUUIDEmpty(t *testing.T) {
	id := EmptyUUID()
	t.Log(id.String())
	max := MaxUUID()
	t.Log(max.String())
	t.Log(max.Compare(id))
}

func TestNewRandomURLToken(t *testing.T) {
	tok1, err := NewRandomURLToken(8)
	if err != nil {
		t.Fatalf("NewRandomURLToken failed, err: %v", err)
	}
	if len(tok1) != 32 {
		t.Errorf("len(tok1) = %d, want 32", len(tok1))
	}

	raw, err := base64.RawURLEncoding.DecodeString(tok1)
	if err != nil {
		t.Errorf("RawURLEncoding.DecodeString(%s) failed, err: %v", tok1, err)
	}
	if len(raw) != 24 {
		t.Errorf("len(raw) = %d, want 24", len(raw))
	}

	tok2, err := NewRandomURLToken(8)
	if err != nil {
		t.Fatalf("NewRandomURLToken failed, err: %v", err)
	}
	if tok1 == tok2 {
		t.Errorf("tok1 == tok2, want different")
	}
	t.Log(tok1)
	t.Log(tok2)
}
