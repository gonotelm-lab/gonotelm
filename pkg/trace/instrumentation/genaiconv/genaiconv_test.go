package genaiconv

import (
	"testing"
)

func TestSpanName(t *testing.T) {
	if got := SpanName("chat"); got != "gen_ai.chat" {
		t.Fatalf("SpanName: got %q want %q", got, "gen_ai.chat")
	}
	if got := SpanName("embedding"); got != "gen_ai.embedding" {
		t.Fatalf("SpanName: got %q want %q", got, "gen_ai.embedding")
	}
}

func TestAttributes(t *testing.T) {
	attrs := Attributes("deepseek", "chat", "deepseek-chat")
	want := map[string]string{
		"gen_ai.operation.name": "chat",
		"gen_ai.system":         "deepseek",
		"gen_ai.request.model":  "deepseek-chat",
	}
	got := make(map[string]string)
	for _, a := range attrs {
		got[string(a.Key)] = a.Value.AsString()
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("attr %s: got %q want %q (attrs=%v)", k, got[k], v, attrs)
		}
	}
}

func TestAttributesOmitsEmptySystemAndModel(t *testing.T) {
	attrs := Attributes("", "chat", "")
	for _, a := range attrs {
		if string(a.Key) == "gen_ai.system" || string(a.Key) == "gen_ai.request.model" {
			t.Fatalf("attr %s should be omitted: %v", a.Key, attrs)
		}
	}
}
