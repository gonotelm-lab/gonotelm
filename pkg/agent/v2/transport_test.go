package agent

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestRunStreamDeltas(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant, ReasoningContent: "用"},
		{Role: schema.Assistant, ReasoningContent: "户"},
		{Role: schema.Assistant, Content: "你"},
		{Role: schema.Assistant, Content: "好", ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop",
			Usage:        tokenUsage(2, 2),
		}},
	}})
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

	res, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks})
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if res.Final.Content != "你好" || res.Final.ReasoningContent != "用户" {
		t.Fatalf("final: %+v", res.Final)
	}
	if res.Usage.TotalTokens != 4 {
		t.Fatalf("usage: %+v", res.Usage)
	}

	want := []string{"rs", "rd:用", "rd:户", "re", "cs", "cd:你", "cd:好", "ce"}
	if len(events) != len(want) {
		t.Fatalf("events: got %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events: got %v, want %v", events, want)
		}
	}
}

func TestRunStreamToolRound(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{chunks: []*schema.Message{
			toolCallAssistant("", newToolCall("c1", "echo", `{"n":1}`)),
		}},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo", run: func(context.Context, string) (string, error) {
			return "tool-ok", nil
		}}},
	})

	res, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if res.Final.Content != "done" || res.ToolCalls != 1 || res.Rounds != 2 {
		t.Fatalf("result: %+v", res)
	}
}

func TestRunStreamEmpty(t *testing.T) {
	ctx := context.Background()
	runner := mustRunner(t, RunnerConfig{Model: newScriptedModel(scriptedResponse{})})

	_, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if !stderrors.Is(err, ErrEmptyResponse) {
		t.Fatalf("run stream: got %v, want ErrEmptyResponse", err)
	}
}

func TestRunStreamHookErrorAborts(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "hello"},
	}})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ContentDelta: func(context.Context, *OutputContext) error {
				return stderrors.New("sink failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunStreamContentFilter(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "partial", ResponseMeta: &schema.ResponseMeta{
			FinishReason: "content_filter",
		}},
	}})
	runner := mustRunner(t, RunnerConfig{Model: model})

	res, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if res.Stop != StopContentFilter || res.Final.Content != "partial" {
		t.Fatalf("result: %+v", res)
	}
}

func TestRunStreamReasoningContentTransition(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant, ReasoningContent: "a"},
		{Role: schema.Assistant, Content: "b"},
		{Role: schema.Assistant, ReasoningContent: "c"},
		{Role: schema.Assistant, Content: "d", ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop",
			Usage:        tokenUsage(1, 1),
		}},
	}})
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

	res, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: hooks})
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if res.Final.ReasoningContent != "ac" || res.Final.Content != "bd" {
		t.Fatalf("final: %+v", res.Final)
	}

	want := []string{"rs", "rd:a", "re", "cs", "cd:b", "ce", "rs", "rd:c", "re", "cs", "cd:d", "ce"}
	if len(events) != len(want) {
		t.Fatalf("events: got %v, want %v", events, want)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events: got %v, want %v", events, want)
		}
	}
}

func TestRunStreamConcatError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant, Content: "a"},
		{Role: schema.User, Content: "b"},
	}})
	runner := mustRunner(t, RunnerConfig{Model: model})

	if _, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err == nil {
		t.Fatal("expected concat error")
	}
}

func TestRunStreamModelError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{err: stderrors.New("stream boom")})
	runner := mustRunner(t, RunnerConfig{Model: model})

	if _, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err == nil {
		t.Fatal("expected stream error")
	}
}

func TestRunStreamSplitToolCall(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{chunks: []*schema.Message{
			partialToolCall("c1", 0, "echo", `{"x":`, ""),
			partialToolCall("c1", 0, "echo", `1}`, "tool_calls"),
		}},
		scriptedResponse{msg: assistantMsg("done", nil)},
	)

	var gotArgs string
	runner := mustRunner(t, RunnerConfig{
		Model: model,
		Tools: []Tool{&fakeTool{name: "echo", run: func(_ context.Context, args string) (string, error) {
			gotArgs = args
			return "ok", nil
		}}},
	})

	res, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if res.ToolCalls != 1 || gotArgs != `{"x":1}` {
		t.Fatalf("tool calls=%d args=%q", res.ToolCalls, gotArgs)
	}
}

type closeProbeModel struct {
	mu     sync.Mutex
	writer *schema.StreamWriter[*schema.Message]
}

var _ einomodel.ToolCallingChatModel = (*closeProbeModel)(nil)

func (m *closeProbeModel) Generate(
	context.Context, []*schema.Message, ...einomodel.Option,
) (*schema.Message, error) {
	return assistantMsg("ok", nil), nil
}

func (m *closeProbeModel) Stream(
	ctx context.Context, in []*schema.Message, opts ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](0)
	m.mu.Lock()
	m.writer = writer
	m.mu.Unlock()

	go func() {
		defer writer.Close()
		writer.Send(assistantMsg("ok", nil), nil)
	}()
	return reader, nil
}

func (m *closeProbeModel) WithTools(
	[]*schema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

func (m *closeProbeModel) lastWriter() *schema.StreamWriter[*schema.Message] {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.writer
}

func probeWriterClosed(w *schema.StreamWriter[*schema.Message]) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = false
		}
	}()
	return w.Send(schema.AssistantMessage("probe", nil), nil)
}

func TestRunStreamClosesReaderEachRound(t *testing.T) {
	ctx := context.Background()
	model := &closeProbeModel{}
	runner := mustRunner(t, RunnerConfig{Model: model, MaxRound: 1})

	if _, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{}); err != nil {
		t.Fatalf("run stream: %v", err)
	}

	writer := model.lastWriter()
	if writer == nil {
		t.Fatal("no stream writer captured")
	}
	if !probeWriterClosed(writer) {
		t.Fatal("stream reader was not closed after the round")
	}
}

func TestRunStreamNilReader(t *testing.T) {
	model := &nilStreamModel{scriptedModel: newScriptedModel()}
	runner := mustRunner(t, RunnerConfig{Model: model})

	if _, err := runner.RunStream(context.Background(), NewSession(struct{}{}), RunConfig[struct{}]{}); err == nil {
		t.Fatal("expected error for nil stream reader")
	}
}

func TestRunStreamReceiveError(t *testing.T) {
	runner := mustRunner(t, RunnerConfig{Model: &recvErrorModel{}})

	if _, err := runner.RunStream(context.Background(), NewSession(struct{}{}), RunConfig[struct{}]{}); err == nil {
		t.Fatal("expected stream receive error")
	}
}

func TestRunStreamCancelDuringReceive(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := mustRunner(t, RunnerConfig{Model: &cancelOnDeltaModel{cancel: cancel}})

	_, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ContentDelta: func(context.Context, *OutputContext) error {
				cancel()
				return nil
			},
		},
	})
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("run stream: got %v, want context.Canceled", err)
	}
}

func TestRunStreamHookErrorStages(t *testing.T) {
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
			model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
				{Role: schema.Assistant, ReasoningContent: "t"},
				{Role: schema.Assistant, Content: "a", ResponseMeta: &schema.ResponseMeta{
					FinishReason: "stop",
				}},
			}})
			runner := mustRunner(t, RunnerConfig{Model: model})

			if _, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{Hooks: tt.hooks}); err == nil {
				t.Fatal("expected output hook error")
			}
		})
	}
}

func TestRunStreamNilChunk(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		nil,
		{Role: schema.Assistant, Content: "ok", ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop",
		}},
	}})
	runner := mustRunner(t, RunnerConfig{Model: model})

	res, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if err != nil {
		t.Fatalf("run stream: %v", err)
	}
	if res.Final.Content != "ok" {
		t.Fatalf("final: %+v", res.Final)
	}
}

func TestRunStreamEmptyChunkMessage(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant},
	}})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{})
	if !stderrors.Is(err, ErrEmptyResponse) {
		t.Fatalf("run stream: got %v, want ErrEmptyResponse", err)
	}
}

func TestRunStreamReasoningOnlyEndError(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(scriptedResponse{chunks: []*schema.Message{
		{Role: schema.Assistant, ReasoningContent: "t", ResponseMeta: &schema.ResponseMeta{
			FinishReason: "stop",
		}},
	}})
	runner := mustRunner(t, RunnerConfig{Model: model})

	_, err := runner.RunStream(ctx, NewSession(struct{}{}), RunConfig[struct{}]{
		Hooks: testHooks{
			ReasoningEnd: func(context.Context, *OutputContext) error {
				return stderrors.New("reasoning end failed")
			},
		},
	})
	if err == nil {
		t.Fatal("expected reasoning end error")
	}
}
