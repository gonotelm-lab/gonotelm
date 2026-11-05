package agent

import (
	stderrors "errors"
	"sync"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestRunBudgetUsageLimit(t *testing.T) {
	b := NewRunBudget(10, 0)

	if err := b.AddUsage(schema.TokenUsage{TotalTokens: 6}); err != nil {
		t.Fatalf("first usage: %v", err)
	}
	if err := b.AddUsage(schema.TokenUsage{TotalTokens: 5}); !stderrors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second usage: got %v, want ErrBudgetExceeded", err)
	}

	unlimited := NewRunBudget(0, 0)
	if err := unlimited.AddUsage(schema.TokenUsage{TotalTokens: 1 << 30}); err != nil {
		t.Fatalf("unlimited usage: %v", err)
	}
}

func TestRunBudgetToolCalls(t *testing.T) {
	b := NewRunBudget(0, 2)

	if err := b.AddToolCall(); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if err := b.AddToolCallFor(2); !stderrors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("batch call: got %v, want ErrBudgetExceeded", err)
	}
}

func TestRunBudgetConcurrent(t *testing.T) {
	b := NewRunBudget(1<<20, 1000)

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			_ = b.AddUsage(schema.TokenUsage{TotalTokens: 1})
			_ = b.AddToolCall()
		})
	}
	wg.Wait()

	if got := b.tokens.Load(); got != 100 {
		t.Fatalf("tokens: got %d, want 100", got)
	}
	if got := b.toolCalls.Load(); got != 100 {
		t.Fatalf("tool calls: got %d, want 100", got)
	}
}

func TestRunBudgetPromptCompletionFallback(t *testing.T) {
	b := NewRunBudget(5, 0)
	err := b.AddUsage(schema.TokenUsage{PromptTokens: 3, CompletionTokens: 3})
	if !stderrors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("usage: got %v, want ErrBudgetExceeded", err)
	}
}

func TestRunBudgetNil(t *testing.T) {
	var b *RunBudget
	if err := b.AddUsage(schema.TokenUsage{TotalTokens: 1}); err != nil {
		t.Fatalf("nil budget usage: %v", err)
	}
	if err := b.AddToolCallFor(2); err != nil {
		t.Fatalf("nil budget tool calls: %v", err)
	}
}

func TestRunBudgetAddZeroToolCalls(t *testing.T) {
	b := NewRunBudget(0, 1)
	if err := b.AddToolCallFor(0); err != nil {
		t.Fatalf("zero tool calls: %v", err)
	}
	if err := b.AddToolCall(); err != nil {
		t.Fatalf("first tool call: %v", err)
	}
	if err := b.AddToolCall(); !stderrors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("second tool call: got %v, want ErrBudgetExceeded", err)
	}
}
