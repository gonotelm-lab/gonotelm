package agent

import (
	"context"
	stderrors "errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestToolPanic(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "boom", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "boom", run: func(context.Context, string) (string, error) {
			panic("kaboom")
		}}},
	})
	session := NewSession(struct{}{})

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	toolMsg := session.Messages[1]
	if !strings.Contains(toolMsg.Content, "tool call panic") {
		t.Fatalf("tool content: %q", toolMsg.Content)
	}
}

func TestToolTimeout(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "slow", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "slow", run: func(ctx context.Context, _ string) (string, error) {
			select {
			case <-time.After(time.Second):
				return "late", nil
			case <-ctx.Done():
				return "", ctx.Err()
			}
		}}},
		Policy: ToolPolicy{Timeout: 10 * time.Millisecond},
	})
	session := NewSession(struct{}{})

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	toolMsg := session.Messages[1]
	if !strings.Contains(toolMsg.Content, "deadline exceeded") {
		t.Fatalf("tool content: %q", toolMsg.Content)
	}
}

func TestToolTruncate(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "long", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "long", run: func(context.Context, string) (string, error) {
			return "0123456789", nil
		}}},
		Policy: ToolPolicy{MaxResultChars: 5},
	})
	session := NewSession(struct{}{})

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := session.Messages[1].Content; !strings.HasPrefix(got, "01234") || !strings.Contains(got, "truncated") {
		t.Fatalf("tool content: %q", got)
	}
}

func TestToolUnknown(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "missing", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{Model: model})
	session := NewSession(struct{}{})

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := session.Messages[1].Content; got != "tool call failed: tool missing not found" {
		t.Fatalf("tool content: %q", got)
	}
}

func TestToolStartHookErrorAborts(t *testing.T) {
	ctx := context.Background()
	var executed atomic.Bool
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo", run: func(context.Context, string) (string, error) {
			executed.Store(true)
			return "ok", nil
		}}},
	})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ToolCallStart: func(context.Context, *ToolCallContext[struct{}]) error {
				return stderrors.New("denied")
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if executed.Load() {
		t.Fatal("tool should not execute after denied hook")
	}
}

func TestToolResultsOrdered(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("",
			newToolCall("c1", "slow", "{}"),
			newToolCall("c2", "fast", "{}"),
		)},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{
			&fakeTool{name: "slow", run: func(context.Context, string) (string, error) {
				time.Sleep(20 * time.Millisecond)
				return "slow-ok", nil
			}},
			&fakeTool{name: "fast", run: func(context.Context, string) (string, error) {
				return "fast-ok", nil
			}},
		},
	})

	var ordered []string
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			RoundComplete: func(_ context.Context, rc *RoundContext[struct{}]) error {
				for _, tr := range rc.Result.ToolResults {
					ordered = append(ordered, tr.Name)
				}
				return nil
			},
		},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if len(ordered) != 2 || ordered[0] != "slow" || ordered[1] != "fast" {
		t.Fatalf("tool order: %v", ordered)
	}
}

func TestToolCallContext(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", `{"x":1}`))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo", run: func(context.Context, string) (string, error) {
			return "out", nil
		}}},
	})

	var (
		startCall ToolCall
		endResult ToolResult
	)
	hooks := testHooks{
		ToolCallStart: func(_ context.Context, tc *ToolCallContext[struct{}]) error {
			startCall = tc.Call
			return nil
		},
		ToolCallEnd: func(_ context.Context, tc *ToolCallContext[struct{}]) error {
			endResult = tc.Result
			return nil
		},
	}

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if startCall.Id != "c1" || startCall.Name != "echo" || startCall.Arguments != `{"x":1}` {
		t.Fatalf("start call: %+v", startCall)
	}
	if endResult.Id != "c1" || endResult.Name != "echo" || endResult.Result != "out" || endResult.Error != nil {
		t.Fatalf("end result: %+v", endResult)
	}
}

func TestToolCallEndHookError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ToolCallEnd: func(context.Context, *ToolCallContext[struct{}]) error {
				return stderrors.New("tool end failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestToolConcurrencyLimit(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("",
			newToolCall("c1", "a", "{}"),
			newToolCall("c2", "b", "{}"),
		)},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)

	var current, maxSeen atomic.Int32
	newSlowTool := func(name string) *fakeTool {
		return &fakeTool{name: name, run: func(context.Context, string) (string, error) {
			n := current.Add(1)
			for {
				m := maxSeen.Load()
				if n <= m || maxSeen.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			current.Add(-1)
			return "ok", nil
		}}
	}

	runner := mustRunner(t, RunnerConfig{
		Model:  model,
		Tools:  []Tool{newSlowTool("a"), newSlowTool("b")},
		Policy: ToolPolicy{MaxConcurrency: 1},
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := maxSeen.Load(); got != 1 {
		t.Fatalf("max concurrency: got %d, want 1", got)
	}
}

func TestToolSerialExecution(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("",
			newToolCall("c1", "a", "{}"),
			newToolCall("c2", "b", "{}"),
			newToolCall("c3", "c", "{}"),
		)},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)

	var (
		order   []string
		current atomic.Int32
		maxSeen atomic.Int32
	)
	newSlowTool := func(name string) *fakeTool {
		return &fakeTool{name: name, run: func(context.Context, string) (string, error) {
			n := current.Add(1)
			for {
				m := maxSeen.Load()
				if n <= m || maxSeen.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(5 * time.Millisecond)
			order = append(order, name)
			current.Add(-1)
			return name, nil
		}}
	}

	runner := mustRunner(t, RunnerConfig{
		Model:  model,
		Tools:  []Tool{newSlowTool("a"), newSlowTool("b"), newSlowTool("c")},
		Policy: ToolPolicy{Serial: true},
	})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := maxSeen.Load(); got != 1 {
		t.Fatalf("max concurrency: got %d, want 1", got)
	}
	if got := strings.Join(order, ","); got != "a,b,c" {
		t.Fatalf("execution order: got %q, want %q", got, "a,b,c")
	}
	if res.ToolCalls != 3 {
		t.Fatalf("tool calls: got %d, want 3", res.ToolCalls)
	}
}

func TestToolSerialContinuesAfterFailure(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("",
			newToolCall("c1", "boom", "{}"),
			newToolCall("c2", "echo", "{}"),
		)},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)

	var ran []string
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{
			&fakeTool{name: "boom", run: func(context.Context, string) (string, error) {
				ran = append(ran, "boom")
				return "", stderrors.New("boom failed")
			}},
			&fakeTool{name: "echo", run: func(context.Context, string) (string, error) {
				ran = append(ran, "echo")
				return "ok", nil
			}},
		},
		Policy: ToolPolicy{Serial: true},
	})

	got := make([]ToolResult, 0, 2)
	hooks := testHooks{
		ToolCallEnd: func(_ context.Context, tc *ToolCallContext[struct{}]) error {
			got = append(got, tc.Result)
			return nil
		},
	}

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final.Content != "done" {
		t.Fatalf("final: %+v", res.Final)
	}
	if strings.Join(ran, ",") != "boom,echo" {
		t.Fatalf("execution order: %v", ran)
	}
	if len(got) != 2 {
		t.Fatalf("tool results: %+v", got)
	}
	if got[0].Error == nil || !strings.Contains(got[0].Result, "boom failed") {
		t.Fatalf("first tool result: %+v", got[0])
	}
	if got[1].Error != nil || got[1].Result != "ok" {
		t.Fatalf("second tool result: %+v", got[1])
	}
}

func TestToolCallEndForUnknownTool(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "missing", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{Model: model})

	var got ToolResult
	hooks := testHooks{
		ToolCallEnd: func(_ context.Context, tc *ToolCallContext[struct{}]) error {
			got = tc.Result
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got.Error == nil || got.Name != "missing" {
		t.Fatalf("tool result: %+v", got)
	}
}

func TestToolCallStartSecondDenied(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("",
			newToolCall("c1", "a", "{}"),
			newToolCall("c2", "b", "{}"),
		)},
	)
	var executed atomic.Int32
	newTool := func(name string) *fakeTool {
		return &fakeTool{name: name, run: func(context.Context, string) (string, error) {
			executed.Add(1)
			return "ok", nil
		}}
	}
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{newTool("a"), newTool("b")},
	})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ToolCallStart: func(_ context.Context, tc *ToolCallContext[struct{}]) error {
				if tc.Call.Name == "b" {
					return stderrors.New("denied")
				}
				return nil
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := executed.Load(); got != 0 {
		t.Fatalf("executed tools: %d, want 0 before batch starts", got)
	}
}

func TestContextCancelDuringTools(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "wait", "{}"))},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "wait", run: func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}}},
	})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ToolCallStart: func(context.Context, *ToolCallContext[struct{}]) error {
				cancel()
				return nil
			},
		},
	})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("run: got %v, want context.Canceled", err)
	}
}

func TestPlanModelOverrideWithTools(t *testing.T) {
	ctx := context.Background()
	base := newScriptedModel(scriptedResponse{msg: assistantMsg("base", nil)})
	override := newScriptedModel(scriptedResponse{msg: assistantMsg("override", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model: base,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.Model = override
			return nil
		},
	}
	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final.Content != "override" {
		t.Fatalf("final: %+v", res.Final)
	}
	calls := override.snapshotCalls()
	if len(calls) != 1 || len(calls[0].tools) != 1 || calls[0].tools[0] != "echo" {
		t.Fatalf("override tools: %+v", calls)
	}
}

func TestPlanToolPolicyTimeout(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "slow", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "slow", run: func(ctx context.Context, _ string) (string, error) {
			<-ctx.Done()
			return "", ctx.Err()
		}}},
	})

	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.ToolPolicy = &ToolPolicy{Timeout: 10 * time.Millisecond}
			return nil
		},
	}
	session := NewSession(struct{}{})
	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := session.Messages[1].Content; !strings.Contains(got, "deadline exceeded") {
		t.Fatalf("tool content: %q", got)
	}
}

func TestToolTruncateUnicode(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "long", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "long", run: func(context.Context, string) (string, error) {
			return "你好世界", nil
		}}},
		Policy: ToolPolicy{MaxResultChars: 2},
	})
	session := NewSession(struct{}{})

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := session.Messages[1].Content; got != "你好\n...[tool result truncated]" {
		t.Fatalf("tool content: %q", got)
	}
}

func TestRunVerboseToolLog(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "long", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "long", run: func(context.Context, string) (string, error) {
			return strings.Repeat("x", 300), nil
		}}},
		Verbose: true,
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
}

func TestToolTruncateNoTruncation(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "short", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "short", run: func(context.Context, string) (string, error) {
			return "ok", nil
		}}},
		Policy: ToolPolicy{MaxResultChars: 5},
	})
	session := NewSession(struct{}{})

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := session.Messages[1].Content; got != "ok" {
		t.Fatalf("tool content: %q", got)
	}
}
