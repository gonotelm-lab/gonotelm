package agent

import (
	"context"
	stderrors "errors"
	"strings"
	"sync"
	"testing"
)

// TestChainHooksRunsInOrder 覆盖「按传入顺序执行」：每个阶段的成员调用顺序都与构造顺序一致。
func TestChainHooksRunsInOrder(t *testing.T) {
	ctx := context.Background()

	var calls []string
	record := func(stage string) func() {
		return func() { calls = append(calls, stage) }
	}

	hooks := NewChainHooks[struct{}](
		testHooks{
			RunStart: func(context.Context, *RunContext[struct{}]) error {
				record("first:run-start")()
				return nil
			},
			RoundStart: func(context.Context, *RoundContext[struct{}]) error {
				record("first:round-start")()
				return nil
			},
			RoundComplete: func(context.Context, *RoundContext[struct{}]) error {
				record("first:round-complete")()
				return nil
			},
			RoundEnd: func(context.Context, *RoundContext[struct{}]) error {
				record("first:round-end")()
				return nil
			},
			RunEnd: func(context.Context, *RunContext[struct{}]) {
				record("first:run-end")()
			},
		},
		testHooks{
			RunStart: func(context.Context, *RunContext[struct{}]) error {
				record("second:run-start")()
				return nil
			},
			RoundStart: func(context.Context, *RoundContext[struct{}]) error {
				record("second:round-start")()
				return nil
			},
			RoundComplete: func(context.Context, *RoundContext[struct{}]) error {
				record("second:round-complete")()
				return nil
			},
			RoundEnd: func(context.Context, *RoundContext[struct{}]) error {
				record("second:round-end")()
				return nil
			},
			RunEnd: func(context.Context, *RunContext[struct{}]) {
				record("second:run-end")()
			},
		},
	)

	runCtx := &RunContext[struct{}]{}
	roundCtx := &RoundContext[struct{}]{}

	if err := hooks.OnRunStart(ctx, runCtx); err != nil {
		t.Fatalf("on run start: %v", err)
	}
	if err := hooks.OnRoundStart(ctx, roundCtx); err != nil {
		t.Fatalf("on round start: %v", err)
	}
	if err := hooks.OnRoundComplete(ctx, roundCtx); err != nil {
		t.Fatalf("on round complete: %v", err)
	}
	if err := hooks.OnRoundEnd(ctx, roundCtx); err != nil {
		t.Fatalf("on round end: %v", err)
	}
	hooks.OnRunEnd(ctx, runCtx)

	want := []string{
		"first:run-start",
		"second:run-start",
		"first:round-start",
		"second:round-start",
		"first:round-complete",
		"second:round-complete",
		"first:round-end",
		"second:round-end",
		"first:run-end",
		"second:run-end",
	}
	if len(calls) != len(want) {
		t.Fatalf("unexpected calls: %v", calls)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("call %d = %q, want %q (all: %v)", i, calls[i], want[i], calls)
		}
	}
}

// TestChainHooksSharesRoundPlan 覆盖「后面的成员能看到前面成员对 RoundPlan 的修改」。
func TestChainHooksSharesRoundPlan(t *testing.T) {
	var sawNoTools bool
	var sawInject int

	hooks := NewChainHooks[struct{}](
		testHooks{
			RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
				rc.Plan.NoTools = true
				rc.Plan.Inject = append(rc.Plan.Inject, nil)
				return nil
			},
		},
		testHooks{
			RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
				sawNoTools = rc.Plan.NoTools
				sawInject = len(rc.Plan.Inject)
				return nil
			},
		},
	)

	roundCtx := &RoundContext[struct{}]{}
	if err := hooks.OnRoundStart(context.Background(), roundCtx); err != nil {
		t.Fatalf("on round start: %v", err)
	}

	if !sawNoTools {
		t.Fatalf("second hook did not see NoTools from the first hook")
	}
	if sawInject != 1 {
		t.Fatalf("second hook did not see injected messages from the first hook: %d", sawInject)
	}
}

// TestChainHooksStopsAtFirstError 覆盖「第一个错误即中止」，并验证错误带上成员序号。
func TestChainHooksStopsAtFirstError(t *testing.T) {
	sentinel := stderrors.New("boom")
	thirdCalled := false

	hooks := NewChainHooks[struct{}](
		testHooks{},
		testHooks{
			RoundStart: func(context.Context, *RoundContext[struct{}]) error {
				return sentinel
			},
		},
		testHooks{
			RoundStart: func(context.Context, *RoundContext[struct{}]) error {
				thirdCalled = true
				return nil
			},
		},
	)

	err := hooks.OnRoundStart(context.Background(), &RoundContext[struct{}]{})
	if !stderrors.Is(err, sentinel) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "chain hook 1 failed") {
		t.Fatalf("error does not carry the member index: %v", err)
	}
	if thirdCalled {
		t.Fatalf("hook after the failing one must not run")
	}
}

// TestChainHooksSkipsNil 覆盖「nil 成员被跳过」：构造时过滤，执行时也不会 panic。
func TestChainHooksSkipsNil(t *testing.T) {
	called := 0
	member := testHooks{
		RoundStart: func(context.Context, *RoundContext[struct{}]) error {
			called++
			return nil
		},
	}

	hooks := NewChainHooks[struct{}](nil, member, nil)
	if err := hooks.OnRoundStart(context.Background(), &RoundContext[struct{}]{}); err != nil {
		t.Fatalf("on round start: %v", err)
	}
	if called != 1 {
		t.Fatalf("unexpected member calls: %d", called)
	}
}

// TestChainHooksEmptyIsNoop 覆盖零参数：所有阶段都不报错、也不 panic。
func TestChainHooksEmptyIsNoop(t *testing.T) {
	ctx := context.Background()
	hooks := NewChainHooks[struct{}]()

	runCtx := &RunContext[struct{}]{}
	roundCtx := &RoundContext[struct{}]{}
	toolCtx := &ToolCallContext[struct{}]{}
	outCtx := &OutputContext{}

	for name, call := range map[string]func() error{
		"run-start":       func() error { return hooks.OnRunStart(ctx, runCtx) },
		"round-start":     func() error { return hooks.OnRoundStart(ctx, roundCtx) },
		"round-complete":  func() error { return hooks.OnRoundComplete(ctx, roundCtx) },
		"round-end":       func() error { return hooks.OnRoundEnd(ctx, roundCtx) },
		"tool-start":      func() error { return hooks.OnToolCallStart(ctx, toolCtx) },
		"tool-end":        func() error { return hooks.OnToolCallEnd(ctx, toolCtx) },
		"reasoning-start": func() error { return hooks.OnReasoningStart(ctx, outCtx) },
		"reasoning-delta": func() error { return hooks.OnReasoningDelta(ctx, outCtx) },
		"reasoning-end":   func() error { return hooks.OnReasoningEnd(ctx, outCtx) },
		"content-start":   func() error { return hooks.OnContentStart(ctx, outCtx) },
		"content-delta":   func() error { return hooks.OnContentDelta(ctx, outCtx) },
		"content-end":     func() error { return hooks.OnContentEnd(ctx, outCtx) },
	} {
		if err := call(); err != nil {
			t.Fatalf("%s on empty chain: %v", name, err)
		}
	}
	hooks.OnRunEnd(ctx, runCtx)
}

// TestChainHooksForwardsRunEndToAll 覆盖无返回值阶段：即使前面的成员在 OnRunStart 失败，
// OnRunEnd 仍然转发给链上所有成员（与引擎对单个 hook 的行为一致）。
func TestChainHooksForwardsRunEndToAll(t *testing.T) {
	var ended []string

	hooks := NewChainHooks[struct{}](
		testHooks{
			RunStart: func(context.Context, *RunContext[struct{}]) error {
				return stderrors.New("start failed")
			},
			RunEnd: func(context.Context, *RunContext[struct{}]) {
				ended = append(ended, "first")
			},
		},
		testHooks{
			RunEnd: func(context.Context, *RunContext[struct{}]) {
				ended = append(ended, "second")
			},
		},
	)

	runCtx := &RunContext[struct{}]{}
	if err := hooks.OnRunStart(context.Background(), runCtx); err == nil {
		t.Fatalf("expected OnRunStart to fail")
	}
	hooks.OnRunEnd(context.Background(), runCtx)

	if len(ended) != 2 || ended[0] != "first" || ended[1] != "second" {
		t.Fatalf("unexpected OnRunEnd delivery: %v", ended)
	}
}

// TestChainHooksToolCallEndConcurrent 覆盖并行工具模式：多个 goroutine 同时转发不会丢调用。
func TestChainHooksToolCallEndConcurrent(t *testing.T) {
	var (
		mu    sync.Mutex
		calls int
	)

	hooks := NewChainHooks[struct{}](
		testHooks{
			ToolCallEnd: func(context.Context, *ToolCallContext[struct{}]) error {
				mu.Lock()
				defer mu.Unlock()
				calls++
				return nil
			},
		},
	)

	const (
		goroutines = 8
		perRoutine = 50
	)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perRoutine; j++ {
				_ = hooks.OnToolCallEnd(context.Background(), &ToolCallContext[struct{}]{})
			}
		}()
	}
	wg.Wait()

	if calls != goroutines*perRoutine {
		t.Fatalf("unexpected concurrent calls: %d", calls)
	}
}
