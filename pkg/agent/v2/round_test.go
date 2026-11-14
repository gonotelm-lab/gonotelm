package agent

import (
	"context"
	stderrors "errors"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func mustRunner(t *testing.T, cfg RunnerConfig) *Runner[struct{}] {
	t.Helper()
	runner, err := NewRunner[struct{}](context.Background(), cfg)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	return runner
}

func TestRunPlainAnswer(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("hello", tokenUsage(3, 2))})
	runner := mustRunner(t, RunnerConfig{Model: model})

	session := NewSession(struct{}{})
	session.System = "sys"
	session.Append(schema.UserMessage("hi"))

	res, err := runner.Run(ctx, session, RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final == nil || res.Final.Content != "hello" {
		t.Fatalf("final: %+v", res.Final)
	}
	if res.Stop != StopFinal || res.Rounds != 1 || res.ToolCalls != 0 {
		t.Fatalf("result: %+v", res)
	}
	if res.Usage.TotalTokens != 5 {
		t.Fatalf("usage: %+v", res.Usage)
	}
	if len(session.Messages) != 2 {
		t.Fatalf("session messages: got %d, want 2", len(session.Messages))
	}
	if session.Usage.TotalTokens != 5 {
		t.Fatalf("session usage: %+v", session.Usage)
	}

	calls := model.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("model calls: got %d", len(calls))
	}
	if len(calls[0].messages) != 2 || calls[0].messages[0].Role != schema.System {
		t.Fatalf("request messages: %+v", calls[0].messages)
	}
}

func TestRunToolRound(t *testing.T) {
	ctx := context.Background()
	echo := &fakeTool{name: "echo", run: func(ctx context.Context, args string) (string, error) {
		return "echo:" + args, nil
	}}
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", `{"x":1}`))},
		scriptedResponse{msg: assistantMsg("done", tokenUsage(5, 5))},
	)
	runner := mustRunner(t, RunnerConfig{Model: model, Tools: []Tool{echo}})

	session := NewSession(struct{}{})
	session.Append(schema.UserMessage("go"))

	res, err := runner.Run(ctx, session, RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final.Content != "done" || res.Rounds != 2 || res.ToolCalls != 1 {
		t.Fatalf("result: %+v", res)
	}
	if len(session.Messages) != 4 {
		t.Fatalf("session messages: got %d, want 4", len(session.Messages))
	}
	toolMsg := session.Messages[2]
	if toolMsg.Role != schema.Tool || toolMsg.ToolCallID != "c1" || toolMsg.ToolName != "echo" {
		t.Fatalf("tool message: %+v", toolMsg)
	}
	if toolMsg.Content != `echo:{"x":1}` {
		t.Fatalf("tool content: %q", toolMsg.Content)
	}

	calls := model.snapshotCalls()
	if len(calls) != 2 {
		t.Fatalf("model calls: got %d", len(calls))
	}
	if len(calls[1].messages) != 3 {
		t.Fatalf("second request messages: got %d, want 3", len(calls[1].messages))
	}
}

func TestRunMaxRounds(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c2", "echo", "{}"))},
	)
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Tools:    []Tool{&fakeTool{name: "echo"}},
		MaxRound: 2,
	})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if !stderrors.Is(err, ErrMaxRounds) {
		t.Fatalf("run: got %v, want ErrMaxRounds", err)
	}
}

func TestRunEarlyStop(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("marker", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			RoundEnd: func(_ context.Context, rc *RoundContext[struct{}]) error {
				rc.Stop = true
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Stop != StopEarly || res.Final.Content != "marker" {
		t.Fatalf("result: %+v", res)
	}
}

func TestRunNilResponseMeta(t *testing.T) {
	ctx := context.Background()
	msg := schema.AssistantMessage("hi", nil)
	model := newScriptedModel(scriptedResponse{msg: msg})
	runner := mustRunner(t, RunnerConfig{Model: model})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final.Content != "hi" || res.Usage.TotalTokens != 0 {
		t.Fatalf("result: %+v", res)
	}
}

func TestRunEmptyResponse(t *testing.T) {
	ctx := context.Background()
	runner := mustRunner(t, RunnerConfig{Model: newScriptedModel(scriptedResponse{})})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if !stderrors.Is(err, ErrEmptyResponse) {
		t.Fatalf("run: got %v, want ErrEmptyResponse", err)
	}
}

func TestRunFinalRound(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: assistantMsg("first", nil)},
		scriptedResponse{msg: assistantMsg("second", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Tools:    []Tool{&fakeTool{name: "echo"}},
		MaxRound: 1,
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{MaxRound: 2}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	calls := model.snapshotCalls()
	if len(calls) != 2 {
		t.Fatalf("model calls: got %d", len(calls))
	}
	if len(calls[0].tools) != 0 {
		t.Fatalf("final round tools: got %v, want none", calls[0].tools)
	}
	if got := calls[0].messages[len(calls[0].messages)-1].Content; got != FinalRoundInstruction {
		t.Fatalf("final instruction: %q", got)
	}
	if len(calls[1].tools) != 1 || calls[1].tools[0] != "echo" {
		t.Fatalf("second run tools: got %v, want [echo]", calls[1].tools)
	}
}

func TestRunFinalRoundKeepTools(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Tools:    []Tool{&fakeTool{name: "echo"}},
		MaxRound: 1,
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: FinalRoundHook[struct{}]{
			KeepTools:   true,
			Instruction: "custom instruction",
		},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	calls := model.snapshotCalls()
	if len(calls[0].tools) != 1 {
		t.Fatalf("tools: got %v, want [echo]", calls[0].tools)
	}
	if got := calls[0].messages[len(calls[0].messages)-1].Content; got != "custom instruction" {
		t.Fatalf("instruction: %q", got)
	}
}

func TestRunNonStreamOutputHooks(t *testing.T) {
	ctx := context.Background()
	msg := assistantMsg("answer", nil)
	msg.ReasoningContent = "think"
	model := newScriptedModel(scriptedResponse{msg: msg})
	runner := mustRunner(t, RunnerConfig{Model: model})

	var events []string
	hooks := testHooks{
		ReasoningStart: func(context.Context, *OutputContext) error { events = append(events, "rs"); return nil },
		ReasoningDelta: func(_ context.Context, oc *OutputContext) error {
			events = append(events, "rd:"+oc.Delta)
			return nil
		},
		ReasoningEnd: func(context.Context, *OutputContext) error { events = append(events, "re"); return nil },
		ContentStart: func(context.Context, *OutputContext) error { events = append(events, "cs"); return nil },
		ContentDelta: func(_ context.Context, oc *OutputContext) error {
			events = append(events, "cd:"+oc.Delta)
			return nil
		},
		ContentEnd: func(context.Context, *OutputContext) error { events = append(events, "ce"); return nil },
	}

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}

	want := []string{"rs", "rd:think", "re", "cs", "cd:answer", "ce"}
	if len(events) != len(want) {
		t.Fatalf("events: got %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events: got %v, want %v", events, want)
		}
	}
}

func TestRunInjectNotPersisted(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})
	session := NewSession(struct{}{})
	session.Append(schema.UserMessage("original"))

	inject := schema.UserMessage("inject")
	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.Inject = []*schema.Message{inject}
			return nil
		},
	}

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}

	calls := model.snapshotCalls()
	if got := calls[0].messages[len(calls[0].messages)-1].Content; got != "inject" {
		t.Fatalf("request inject: %q", got)
	}
	for _, msg := range session.Messages {
		if msg.Content == "inject" {
			t.Fatalf("inject leaked into session: %+v", session.Messages)
		}
	}
}

func TestRunUsageDeltaAndCumulative(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: assistantMsg("a", tokenUsage(3, 2))},
		scriptedResponse{msg: assistantMsg("b", tokenUsage(4, 3))},
	)
	runner := mustRunner(t, RunnerConfig{Model: model})
	session := NewSession(struct{}{})

	first, err := runner.Run(ctx, session, RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := runner.Run(ctx, session, RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if first.Usage.TotalTokens != 5 || second.Usage.TotalTokens != 7 {
		t.Fatalf("run usage: first=%+v second=%+v", first.Usage, second.Usage)
	}
	if session.Usage.TotalTokens != 12 {
		t.Fatalf("session usage: %+v", session.Usage)
	}
}

func TestRunOnRunEndOnError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{err: stderrors.New("boom")})
	runner := mustRunner(t, RunnerConfig{Model: model})

	var (
		called bool
		gotErr error
		gotRes *Result
	)
	hooks := testHooks{
		RunEnd: func(_ context.Context, rc *RunContext[struct{}]) {
			called = true
			gotRes = rc.Result
			gotErr = rc.Err
		},
	}

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err == nil {
		t.Fatal("expected error")
	}
	if !called || gotErr == nil || gotRes != nil {
		t.Fatalf("on run end: called=%v err=%v res=%+v", called, gotErr, gotRes)
	}
}

func TestRunBudgetExceeded(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("a", tokenUsage(3, 3))})
	runner := mustRunner(t, RunnerConfig{Model: model})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Budget: NewRunBudget(3, 0),
	})
	if !stderrors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("run: got %v, want ErrBudgetExceeded", err)
	}
	if res == nil || res.Usage.TotalTokens != 6 {
		t.Fatalf("result: %+v", res)
	}
}

func TestRunRoundStartCompaction(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})
	session := NewSession(struct{}{})
	session.Append(schema.UserMessage("original"))

	compacted := schema.UserMessage("compacted")
	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Session.Replace([]*schema.Message{compacted})
			return nil
		},
	}

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}

	if got := model.snapshotCalls()[0].messages[0].Content; got != "compacted" {
		t.Fatalf("request: %q", got)
	}
	if len(session.Messages) != 2 || session.Messages[0].Content != "compacted" {
		t.Fatalf("session: %+v", session.Messages)
	}
}

func TestRunExtraTools(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "extra", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{Model: model})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		ExtraTools: []Tool{&fakeTool{name: "extra", run: func(context.Context, string) (string, error) {
			return "extra-result", nil
		}}},
	})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.ToolCalls != 1 {
		t.Fatalf("result: %+v", res)
	}
	if got := model.snapshotCalls()[0].tools; len(got) != 1 || got[0] != "extra" {
		t.Fatalf("tools: %v", got)
	}
}

func TestRunWithoutTools(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{WithoutTools: true}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := model.snapshotCalls()[0].tools; len(got) != 0 {
		t.Fatalf("tools: got %v, want none", got)
	}
}

func TestRunContentFilter(t *testing.T) {
	ctx := context.Background()
	msg := assistantMsg("partial", nil)
	msg.ResponseMeta.FinishReason = "content_filter"
	model := newScriptedModel(scriptedResponse{msg: msg})
	runner := mustRunner(t, RunnerConfig{Model: model})
	session := NewSession(struct{}{})

	res, err := runner.Run(ctx, session, RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Stop != StopContentFilter || res.Final.Content != "partial" {
		t.Fatalf("result: %+v", res)
	}
	if len(session.Messages) != 1 {
		t.Fatalf("session: %+v", session.Messages)
	}
}

func TestRoundPlanToolsOverride(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "custom", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "static"}},
	})

	var used bool
	custom := &fakeTool{name: "custom", run: func(context.Context, string) (string, error) {
		used = true
		return "ok", nil
	}}
	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.Tools = []Tool{custom}
			return nil
		},
	}

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !used {
		t.Fatal("custom tool was not executed")
	}
	calls := model.snapshotCalls()
	if got := calls[0].tools; len(got) != 1 || got[0] != "custom" {
		t.Fatalf("tools: %v, want [custom]", got)
	}
}

func TestRoundPlanToolPolicyOverride(t *testing.T) {
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
	})

	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.ToolPolicy = &ToolPolicy{MaxResultChars: 5}
			return nil
		},
	}
	session := NewSession(struct{}{})
	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := session.Messages[1].Content; got != "01234\n...[tool result truncated]" {
		t.Fatalf("tool content: %q", got)
	}
}

func TestRoundPlanModelOverride(t *testing.T) {
	ctx := context.Background()
	base := newScriptedModel(scriptedResponse{msg: assistantMsg("base", nil)})
	override := newScriptedModel(scriptedResponse{msg: assistantMsg("override", nil)})
	runner := mustRunner(t, RunnerConfig{Model: base})

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
	if len(base.snapshotCalls()) != 0 {
		t.Fatal("base model should not be called")
	}
	if len(override.snapshotCalls()) != 1 {
		t.Fatalf("override calls: %d", len(override.snapshotCalls()))
	}
}

func TestRunInfoMaxRound(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model, MaxRound: 7})

	var got int
	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			got = rc.Info.MaxRound
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got != 7 {
		t.Fatalf("max round: got %d, want 7", got)
	}
}

func TestRunContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("run: got %v, want context.Canceled", err)
	}
}

func TestRunStartHookError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	var (
		ended  bool
		endErr error
	)
	hooks := testHooks{
		RunStart: func(context.Context, *RunContext[struct{}]) error {
			return stderrors.New("start failed")
		},
		RunEnd: func(_ context.Context, rc *RunContext[struct{}]) {
			ended = true
			endErr = rc.Err
		},
	}

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks})
	if err == nil {
		t.Fatal("expected error")
	}
	if !ended || endErr == nil {
		t.Fatalf("on run end: ended=%v err=%v", ended, endErr)
	}
}

func TestRoundStartHookError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			RoundStart: func(context.Context, *RoundContext[struct{}]) error {
				return stderrors.New("round start failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRoundCompleteHookError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			RoundComplete: func(context.Context, *RoundContext[struct{}]) error {
				return stderrors.New("round complete failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRoundEndHookError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			RoundEnd: func(context.Context, *RoundContext[struct{}]) error {
				return stderrors.New("round end failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPlanNoToolsWins(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.Tools = []Tool{&fakeTool{name: "echo"}}
			rc.Plan.NoTools = true
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := model.snapshotCalls()[0].tools; len(got) != 0 {
		t.Fatalf("tools: got %v, want none", got)
	}
}

func TestPlanToolsValidationError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.Tools = []Tool{
				&fakeTool{name: "dup"},
				&fakeTool{name: "dup"},
			}
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err == nil {
		t.Fatal("expected duplicate tool error")
	}
}

func TestRunBudgetToolCallsExceeded(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("",
			newToolCall("c1", "echo", "{}"),
			newToolCall("c2", "echo", "{}"),
		)},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Budget: NewRunBudget(0, 1),
	})
	if !stderrors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("run: got %v, want ErrBudgetExceeded", err)
	}
	if res == nil || res.ToolCalls != 0 {
		t.Fatalf("result: %+v", res)
	}
}

func TestFinalRoundHookDisable(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Tools:    []Tool{&fakeTool{name: "echo"}},
		MaxRound: 1,
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: FinalRoundHook[struct{}]{Disable: true},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}

	calls := model.snapshotCalls()
	if got := calls[0].tools; len(got) != 1 || got[0] != "echo" {
		t.Fatalf("tools: got %v, want [echo]", got)
	}
	for _, msg := range calls[0].messages {
		if msg.Content == FinalRoundInstruction {
			t.Fatal("final instruction should not be injected when disabled")
		}
	}
}

func TestRunNilSession(t *testing.T) {
	runner := mustRunner(t, RunnerConfig{Model: newScriptedModel()})
	if _, err := runner.Run(context.Background(), nil, RunConfig[struct{}]{}); err == nil {
		t.Fatal("expected error for nil session")
	}
}

func TestWithoutToolsWinsOverPlanTools(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	hooks := testHooks{
		RoundStart: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Plan.Tools = []Tool{&fakeTool{name: "echo"}}
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks:        hooks,
		WithoutTools: true,
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got := model.snapshotCalls()[0].tools; len(got) != 0 {
		t.Fatalf("tools: got %v, want none", got)
	}
}

func TestRunExtraToolsConflictWithStatic(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		ExtraTools: []Tool{&fakeTool{name: "echo"}},
	})
	if err == nil {
		t.Fatal("expected duplicate tool error")
	}
}

func TestFinalRoundHookDefaultAfterToolRound(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Tools:    []Tool{&fakeTool{name: "echo"}},
		MaxRound: 2,
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	calls := model.snapshotCalls()
	if len(calls) != 2 {
		t.Fatalf("model calls: got %d", len(calls))
	}
	if got := calls[0].tools; len(got) != 1 || got[0] != "echo" {
		t.Fatalf("first round tools: %v", got)
	}
	if got := calls[1].tools; len(got) != 0 {
		t.Fatalf("final round tools: got %v, want none", got)
	}
	if got := calls[1].messages[len(calls[1].messages)-1].Content; got != FinalRoundInstruction {
		t.Fatalf("final instruction: %q", got)
	}
}

func TestRunConfigModelOverride(t *testing.T) {
	ctx := context.Background()
	base := newScriptedModel(scriptedResponse{msg: assistantMsg("base", nil)})
	override := newScriptedModel(scriptedResponse{msg: assistantMsg("override", nil)})
	runner := mustRunner(t, RunnerConfig{Model: base})

	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Model: override})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Final.Content != "override" {
		t.Fatalf("final: %+v", res.Final)
	}
	if len(base.snapshotCalls()) != 0 || len(override.snapshotCalls()) != 1 {
		t.Fatalf("base calls=%d override calls=%d", len(base.snapshotCalls()), len(override.snapshotCalls()))
	}
}

func TestRunEndReceivesResult(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model, MaxRound: 3})

	var got *Result
	hooks := testHooks{
		RunEnd: func(_ context.Context, rc *RunContext[struct{}]) {
			got = rc.Result
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if got == nil || got.Final.Content != "ok" || got.Rounds != 1 {
		t.Fatalf("run end result: %+v", got)
	}
}

func TestSessionResetThenRun(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: assistantMsg("a", tokenUsage(1, 1))},
		scriptedResponse{msg: assistantMsg("b", nil)},
	)
	runner := mustRunner(t, RunnerConfig{Model: model})
	session := NewSession(struct{}{})
	session.Append(schema.UserMessage("first"))

	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	session.Reset()

	if len(session.Messages) != 0 || session.Usage.TotalTokens != 0 {
		t.Fatalf("reset session: %+v", session)
	}
	if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(session.Messages) != 1 || session.Messages[0].Content != "b" {
		t.Fatalf("second run messages: %+v", session.Messages)
	}
}

func TestRunToolCallNoOutputHooks(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
		scriptedResponse{msg: assistantMsg("", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	var events []string
	hooks := testHooks{
		ReasoningStart: func(context.Context, *OutputContext) error { events = append(events, "rs"); return nil },
		ReasoningDelta: func(context.Context, *OutputContext) error { return nil },
		ReasoningEnd:   func(context.Context, *OutputContext) error { return nil },
		ContentStart:   func(context.Context, *OutputContext) error { events = append(events, "cs"); return nil },
		ContentDelta:   func(context.Context, *OutputContext) error { return nil },
		ContentEnd:     func(context.Context, *OutputContext) error { events = append(events, "ce"); return nil },
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("output events for empty content: %v", events)
	}
}

func TestRoundEndAfterToolRound(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	hooks := testHooks{
		RoundEnd: func(_ context.Context, rc *RoundContext[struct{}]) error {
			rc.Stop = true
			return nil
		},
	}
	res, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.Stop != StopEarly {
		t.Fatalf("stop: %v", res.Stop)
	}
	if res.Final.Role != schema.Assistant || len(res.Final.ToolCalls) != 1 {
		t.Fatalf("final: %+v", res.Final)
	}
}

func TestRunStartContext(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Name:     "test-agent",
		MaxRound: 4,
	})

	var (
		agent      string
		maxRound   int
		hasSession bool
	)
	hooks := testHooks{
		RunStart: func(_ context.Context, rc *RunContext[struct{}]) error {
			agent = rc.Info.Agent
			maxRound = rc.Info.MaxRound
			hasSession = rc.Session != nil
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if agent != "test-agent" || maxRound != 4 || !hasSession {
		t.Fatalf("run start context: agent=%q maxRound=%d hasSession=%v", agent, maxRound, hasSession)
	}
}

func TestRunConfigMaxRoundOverride(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model:    model,
		Tools:    []Tool{&fakeTool{name: "echo"}},
		MaxRound: 10,
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{MaxRound: 1}); err != nil {
		t.Fatalf("run: %v", err)
	}
	calls := model.snapshotCalls()
	if got := calls[0].tools; len(got) != 0 {
		t.Fatalf("tools: got %v, want none", got)
	}
	if got := calls[0].messages[len(calls[0].messages)-1].Content; got != FinalRoundInstruction {
		t.Fatalf("final instruction: %q", got)
	}
}

func TestExtraToolsSortedWithStatic(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "c"}},
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		ExtraTools: []Tool{&fakeTool{name: "b"}, &fakeTool{name: "a"}},
	}); err != nil {
		t.Fatalf("run: %v", err)
	}
	calls := model.snapshotCalls()
	want := []string{"a", "b", "c"}
	if len(calls[0].tools) != len(want) {
		t.Fatalf("tools: got %v, want %v", calls[0].tools, want)
	}
	for i := range want {
		if calls[0].tools[i] != want[i] {
			t.Fatalf("tools: got %v, want %v", calls[0].tools, want)
		}
	}
}

func TestRoundCompleteEmptyToolResults(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	var got RoundResult
	hooks := testHooks{
		RoundComplete: func(_ context.Context, rc *RoundContext[struct{}]) error {
			got = rc.Result
			return nil
		},
	}
	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(got.ToolResults) != 0 || len(got.NewMessages) != 1 || got.Assistant.Content != "ok" {
		t.Fatalf("round result: %+v", got)
	}
}

func TestNoopHooks(t *testing.T) {
	ctx := context.Background()
	msg := assistantMsg("answer", nil)
	msg.ReasoningContent = "think"
	model := newScriptedModel(
		scriptedResponse{msg: toolCallAssistant("", newToolCall("c1", "echo", "{}"))},
		scriptedResponse{msg: msg},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo"}},
	})

	if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: NoopHooks[struct{}]{},
	}); err != nil {
		t.Fatalf("run with noop hooks: %v", err)
	}
}

func TestRunOutputHookErrorNonStream(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("answer", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ContentDelta: func(context.Context, *OutputContext) error {
				return stderrors.New("delta failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected output hook error")
	}
}

func TestExtraToolsInfoError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{msg: assistantMsg("ok", nil)})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		ExtraTools: []Tool{&fakeTool{name: "bad", infoErr: stderrors.New("info failed")}},
	})
	if err == nil {
		t.Fatal("expected extra tool info error")
	}
}

func TestRunOutputHookErrorStages(t *testing.T) {
	ctx := context.Background()
	fail := func(context.Context, *OutputContext) error {
		return stderrors.New("output hook failed")
	}

	tests := []struct {
		name  string
		hooks testHooks
	}{
		{name: "reasoning start", hooks: testHooks{ReasoningStart: fail}},
		{name: "reasoning delta", hooks: testHooks{ReasoningDelta: fail}},
		{name: "reasoning end", hooks: testHooks{ReasoningEnd: fail}},
		{name: "content start", hooks: testHooks{ContentStart: fail}},
		{name: "content delta", hooks: testHooks{ContentDelta: fail}},
		{name: "content end", hooks: testHooks{ContentEnd: fail}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := assistantMsg("answer", nil)
			msg.ReasoningContent = "think"
			model := newScriptedModel(scriptedResponse{msg: msg})
			runner := mustRunner(t, RunnerConfig{Model: model})

			if _, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: tt.hooks}); err == nil {
				t.Fatal("expected output hook error")
			}
		})
	}
}

func TestRunContentFilterHookErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("round complete error", func(t *testing.T) {
		msg := assistantMsg("partial", nil)
		msg.ResponseMeta.FinishReason = "content_filter"
		model := newScriptedModel(scriptedResponse{msg: msg})
		runner := mustRunner(t, RunnerConfig{Model: model})

		_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
			Hooks: testHooks{
				RoundComplete: func(context.Context, *RoundContext[struct{}]) error {
					return stderrors.New("complete failed")
				},
			},
		})
		if err == nil {
			t.Fatal("expected round complete error")
		}
	})

	t.Run("round end error", func(t *testing.T) {
		msg := assistantMsg("partial", nil)
		msg.ResponseMeta.FinishReason = "content_filter"
		model := newScriptedModel(scriptedResponse{msg: msg})
		runner := mustRunner(t, RunnerConfig{Model: model})

		_, err := runner.Run(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
			Hooks: testHooks{
				RoundEnd: func(context.Context, *RoundContext[struct{}]) error {
					return stderrors.New("end failed")
				},
			},
		})
		if err == nil {
			t.Fatal("expected round end error")
		}
	})
}
