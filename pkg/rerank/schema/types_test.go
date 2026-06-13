package schema

import (
	"encoding/json"
	"testing"
)

func TestQueryMarshal(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		q := NewStringQuery("hello")
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("marshal string query failed: %v", err)
		}
		if got := string(b); got != `"hello"` {
			t.Fatalf("unexpected marshaled string query: %s", got)
		}
	})

	t.Run("object_text", func(t *testing.T) {
		q := NewTextQuery("你好")
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("marshal object query failed: %v", err)
		}
		if got := string(b); got != `{"text":"你好"}` {
			t.Fatalf("unexpected marshaled object query: %s", got)
		}
	})

	t.Run("object_image", func(t *testing.T) {
		q := NewImageQuery("https://img.example.com/a.png")
		b, err := json.Marshal(q)
		if err != nil {
			t.Fatalf("marshal image query failed: %v", err)
		}
		if got := string(b); got != `{"image":"https://img.example.com/a.png"}` {
			t.Fatalf("unexpected marshaled image query: %s", got)
		}
	})
}

func TestQueryUnmarshal(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		var q Query
		if err := json.Unmarshal([]byte(`"hello"`), &q); err != nil {
			t.Fatalf("unmarshal string query failed: %v", err)
		}
		if !q.IsString() || q.String != "hello" || q.Object != nil {
			t.Fatalf("unexpected query value: %+v", q)
		}
	})

	t.Run("object", func(t *testing.T) {
		var q Query
		if err := json.Unmarshal([]byte(`{"image":"img-b64-or-url"}`), &q); err != nil {
			t.Fatalf("unmarshal object query failed: %v", err)
		}
		if !q.IsObject() || q.Object == nil || q.Object.Image != "img-b64-or-url" {
			t.Fatalf("unexpected query value: %+v", q)
		}
	})
}
