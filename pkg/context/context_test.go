package context

import (
	"context"
	"log/slog"
	"testing"
)

func TestSubagentContext(t *testing.T) {
	ctx := context.Background()
	if IsSubagent(ctx) {
		t.Fatal("empty context should not be marked as subagent")
	}

	ctx = WithSubagent(ctx)
	if !IsSubagent(ctx) {
		t.Fatal("subagent context not detected")
	}

	attrs := ToSlogAttrs(ctx)
	var found bool
	for _, attr := range attrs {
		if attr.Key != AttrKeySubagent {
			continue
		}
		found = true
		if attr.Value.Kind() != slog.KindBool || !attr.Value.Bool() {
			t.Fatalf("subagent slog attr = %v, want true", attr.Value)
		}
	}
	if !found {
		t.Fatalf("subagent slog attr missing: %v", attrs)
	}
}

func TestSubagentContextNotLeaked(t *testing.T) {
	parent := context.Background()
	_ = WithSubagent(parent)

	if IsSubagent(parent) {
		t.Fatal("parent context must not be mutated")
	}
}
