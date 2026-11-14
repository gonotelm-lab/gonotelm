package agent

import (
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestSessionAppendReplaceCloneReset(t *testing.T) {
	s := &Session[struct{}]{
		System:   "sys",
		Messages: []*schema.Message{schema.UserMessage("a")},
		Usage:    schema.TokenUsage{TotalTokens: 3},
	}

	s.Append(schema.UserMessage("b"))
	if len(s.Messages) != 2 {
		t.Fatalf("append: got %d messages, want 2", len(s.Messages))
	}

	clone := s.Clone()
	clone.Append(schema.UserMessage("c"))
	if len(s.Messages) != 2 {
		t.Fatalf("clone mutated original: got %d messages, want 2", len(s.Messages))
	}
	if len(clone.Messages) != 3 {
		t.Fatalf("clone append: got %d messages, want 3", len(clone.Messages))
	}

	s.Replace([]*schema.Message{schema.UserMessage("replaced")})
	if len(s.Messages) != 1 || s.Messages[0].Content != "replaced" {
		t.Fatalf("replace: got %+v", s.Messages)
	}

	s.Reset()
	if len(s.Messages) != 0 {
		t.Fatalf("reset: got %d messages, want 0", len(s.Messages))
	}
	if s.Usage.TotalTokens != 0 {
		t.Fatalf("reset usage: got %d, want 0", s.Usage.TotalTokens)
	}
	if s.System != "sys" {
		t.Fatalf("reset system: got %q, want sys", s.System)
	}
}

func TestSessionAppendEmpty(t *testing.T) {
	s := &Session[struct{}]{}
	s.Append()
	if len(s.Messages) != 0 {
		t.Fatalf("append empty: got %d messages", len(s.Messages))
	}
}
