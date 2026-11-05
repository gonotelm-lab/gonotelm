package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type fakeSubagentResponse struct {
	msg *schema.Message
	err error
}

type fakeSubagentCall struct {
	messages []*schema.Message
	tools    []string
}

type fakeSubagentState struct {
	mu        sync.Mutex
	responses []fakeSubagentResponse
	index     int
	calls     []fakeSubagentCall
}

type fakeSubagentModel struct {
	state *fakeSubagentState
	tools []*schema.ToolInfo
}

var _ model.ToolCallingChatModel = (*fakeSubagentModel)(nil)

func newFakeSubagentModel(responses ...fakeSubagentResponse) *fakeSubagentModel {
	return &fakeSubagentModel{state: &fakeSubagentState{responses: responses}}
}

func (m *fakeSubagentModel) Generate(
	ctx context.Context,
	in []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	tools := make([]string, 0, len(m.tools))
	for _, info := range m.tools {
		tools = append(tools, info.Name)
	}
	m.state.calls = append(m.state.calls, fakeSubagentCall{
		messages: append([]*schema.Message(nil), in...),
		tools:    tools,
	})

	if m.state.index >= len(m.state.responses) {
		return nil, fmt.Errorf("unexpected model call #%d", m.state.index)
	}
	resp := m.state.responses[m.state.index]
	m.state.index++
	return resp.msg, resp.err
}

func (m *fakeSubagentModel) Stream(
	ctx context.Context,
	in []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(msg, nil)
	}()
	return reader, nil
}

func (m *fakeSubagentModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &fakeSubagentModel{state: m.state, tools: tools}, nil
}

func (m *fakeSubagentModel) snapshotCalls() []fakeSubagentCall {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	out := make([]fakeSubagentCall, len(m.state.calls))
	copy(out, m.state.calls)
	return out
}

type constantSubagentModel struct {
	content string
}

var _ model.ToolCallingChatModel = (*constantSubagentModel)(nil)

func (m *constantSubagentModel) Generate(
	context.Context,
	[]*schema.Message,
	...model.Option,
) (*schema.Message, error) {
	return subagentAssistantMsg(m.content), nil
}

func (m *constantSubagentModel) Stream(
	context.Context,
	[]*schema.Message,
	...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(subagentAssistantMsg(m.content), nil)
	}()
	return reader, nil
}

func (m *constantSubagentModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type blockingSubagentModel struct{}

var _ model.ToolCallingChatModel = (*blockingSubagentModel)(nil)

func (m *blockingSubagentModel) Generate(
	ctx context.Context,
	_ []*schema.Message,
	_ ...model.Option,
) (*schema.Message, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

func (m *blockingSubagentModel) Stream(
	ctx context.Context,
	in []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(msg, nil)
	}()
	return reader, nil
}

func (m *blockingSubagentModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

type fakeSubagentTool struct {
	name    string
	infoErr error
	nilInfo bool
	run     func(ctx context.Context, args string) (string, error)
}

var _ tool.InvokableTool = (*fakeSubagentTool)(nil)

func (t *fakeSubagentTool) Info(context.Context) (*schema.ToolInfo, error) {
	if t.infoErr != nil {
		return nil, t.infoErr
	}
	if t.nilInfo {
		return nil, nil
	}
	return &schema.ToolInfo{Name: t.name, Desc: t.name}, nil
}

func (t *fakeSubagentTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	if t.run == nil {
		return "ok", nil
	}
	return t.run(ctx, args)
}

func subagentAssistantMsg(content string) *schema.Message {
	msg := schema.AssistantMessage(content, nil)
	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: "stop",
		Usage:        &schema.TokenUsage{},
	}
	return msg
}

func subagentToolCallMsg(callID, name, args string) *schema.Message {
	msg := schema.AssistantMessage("", []schema.ToolCall{{
		ID:   callID,
		Type: "function",
		Function: schema.FunctionCall{
			Name:      name,
			Arguments: args,
		},
	}})
	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: "tool_calls",
		Usage:        &schema.TokenUsage{},
	}
	return msg
}

func subagentArgs(t *testing.T, name, title, prompt string) string {
	t.Helper()
	b, err := json.Marshal(SubagentToolInput{Name: name, Title: title, Prompt: prompt})
	if err != nil {
		t.Fatalf("marshal subagent args: %v", err)
	}
	return string(b)
}

func TestNewSubagentToolValidation(t *testing.T) {
	t.Run("nil base llm", func(t *testing.T) {
		if _, err := NewSubagentTool(context.Background(), SubagentConfig{}); err == nil {
			t.Fatal("expected nil base llm error")
		}
	})

	t.Run("recursion", func(t *testing.T) {
		_, err := NewSubagentTool(context.Background(), SubagentConfig{
			BaseLLM: newFakeSubagentModel(),
			Tools: map[string]tool.InvokableTool{
				SubagentToolName: &fakeSubagentTool{name: SubagentToolName},
			},
		})
		if err == nil {
			t.Fatal("expected recursion error")
		}
	})

	t.Run("nil tool", func(t *testing.T) {
		_, err := NewSubagentTool(context.Background(), SubagentConfig{
			BaseLLM: newFakeSubagentModel(),
			Tools:   map[string]tool.InvokableTool{"nil": nil},
		})
		if err == nil {
			t.Fatal("expected nil tool error")
		}
	})

	t.Run("name mismatch", func(t *testing.T) {
		_, err := NewSubagentTool(context.Background(), SubagentConfig{
			BaseLLM: newFakeSubagentModel(),
			Tools: map[string]tool.InvokableTool{
				"a": &fakeSubagentTool{name: "b"},
			},
		})
		if err == nil {
			t.Fatal("expected name mismatch error")
		}
	})

	t.Run("info error", func(t *testing.T) {
		_, err := NewSubagentTool(context.Background(), SubagentConfig{
			BaseLLM: newFakeSubagentModel(),
			Tools: map[string]tool.InvokableTool{
				"a": &fakeSubagentTool{name: "a", infoErr: fmt.Errorf("info failed")},
			},
		})
		if err == nil {
			t.Fatal("expected info error")
		}
	})

	t.Run("nil info", func(t *testing.T) {
		_, err := NewSubagentTool(context.Background(), SubagentConfig{
			BaseLLM: newFakeSubagentModel(),
			Tools: map[string]tool.InvokableTool{
				"a": &fakeSubagentTool{name: "a", nilInfo: true},
			},
		})
		if err == nil {
			t.Fatal("expected nil info error")
		}
	})

	t.Run("runner validation error", func(t *testing.T) {
		_, err := NewSubagentTool(context.Background(), SubagentConfig{
			BaseLLM: newFakeSubagentModel(),
			Tools: map[string]tool.InvokableTool{
				"": &fakeSubagentTool{name: ""},
			},
		})
		if err == nil {
			t.Fatal("expected runner validation error")
		}
	})
}

func TestSubagentToolInfo(t *testing.T) {
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{BaseLLM: newFakeSubagentModel()})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	info, err := tl.Info(context.Background())
	if err != nil {
		t.Fatalf("info: %v", err)
	}
	if info.Name != SubagentToolName || info.ParamsOneOf == nil {
		t.Fatalf("info: %+v", info)
	}
}

func TestSubagentInvokableRunInputValidation(t *testing.T) {
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{BaseLLM: newFakeSubagentModel()})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	tests := []struct {
		name string
		args string
	}{
		{name: "invalid json", args: `{`},
		{name: "missing name", args: `{"title":"t","prompt":"p"}`},
		{name: "missing title", args: `{"name":"n","prompt":"p"}`},
		{name: "missing prompt", args: `{"name":"n","title":"t"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tl.InvokableRun(context.Background(), tt.args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestSubagentInvokableRunHappyPath(t *testing.T) {
	model := newFakeSubagentModel(fakeSubagentResponse{msg: subagentAssistantMsg("answer")})
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM: model,
		Tools: map[string]tool.InvokableTool{
			"echo": &fakeSubagentTool{name: "echo"},
		},
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	got, err := tl.InvokableRun(context.Background(), subagentArgs(t, "researcher", "look up facts", "find the answer"))
	if err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	if !strings.Contains(got, "<subagent_result name=\"researcher\" title=\"look up facts\">") ||
		!strings.Contains(got, "answer") {
		t.Fatalf("result: %q", got)
	}

	calls := model.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("model calls: %d", len(calls))
	}
	if len(calls[0].messages) < 2 {
		t.Fatalf("messages: %+v", calls[0].messages)
	}
	if calls[0].messages[0].Role != schema.System ||
		!strings.Contains(calls[0].messages[0].Content, "researcher") ||
		!strings.Contains(calls[0].messages[0].Content, "look up facts") {
		t.Fatalf("system prompt: %+v", calls[0].messages[0])
	}
	if got := calls[0].messages[len(calls[0].messages)-1].Content; got != "find the answer" {
		t.Fatalf("user prompt: %q", got)
	}
}

func TestSubagentDefaultSystemPrompt(t *testing.T) {
	model := newFakeSubagentModel(fakeSubagentResponse{msg: subagentAssistantMsg("answer")})
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{BaseLLM: model})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	if _, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p")); err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	calls := model.snapshotCalls()
	if got := calls[0].messages[0].Content; !strings.Contains(got, "You are a subagent") {
		t.Fatalf("default system prompt: %q", got)
	}
}

func TestSubagentToolRound(t *testing.T) {
	model := newFakeSubagentModel(
		fakeSubagentResponse{msg: subagentToolCallMsg("c1", "echo", `{"x":1}`)},
		fakeSubagentResponse{msg: subagentAssistantMsg("done")},
	)
	var executed bool
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM: model,
		Tools: map[string]tool.InvokableTool{
			"echo": &fakeSubagentTool{name: "echo", run: func(context.Context, string) (string, error) {
				executed = true
				return "tool-result", nil
			}},
		},
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	got, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p"))
	if err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	if !executed || !strings.Contains(got, "done") {
		t.Fatalf("executed=%v result=%q", executed, got)
	}
	calls := model.snapshotCalls()
	if len(calls) != 2 || len(calls[0].tools) != 1 || calls[0].tools[0] != "echo" {
		t.Fatalf("model calls: %+v", calls)
	}
}

func TestSubagentFinalRound(t *testing.T) {
	model := newFakeSubagentModel(fakeSubagentResponse{msg: subagentAssistantMsg("done")})
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM:  model,
		MaxRound: 1,
		Tools: map[string]tool.InvokableTool{
			"echo": &fakeSubagentTool{name: "echo"},
		},
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	if _, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p")); err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	calls := model.snapshotCalls()
	if len(calls) != 1 {
		t.Fatalf("model calls: %d", len(calls))
	}
	if len(calls[0].tools) != 0 {
		t.Fatalf("final round tools: %v", calls[0].tools)
	}
	var foundInstruction bool
	for _, msg := range calls[0].messages {
		if strings.Contains(strings.ToLower(msg.Content), "final round") {
			foundInstruction = true
		}
	}
	if !foundInstruction {
		t.Fatalf("final round instruction missing: %+v", calls[0].messages)
	}
}

func TestSubagentCallLimit(t *testing.T) {
	model := newFakeSubagentModel(
		fakeSubagentResponse{msg: subagentAssistantMsg("one")},
		fakeSubagentResponse{msg: subagentAssistantMsg("two")},
	)
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM:        model,
		MaxCallsPerRun: 1,
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	if _, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p")); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p2")); err == nil {
		t.Fatal("expected call limit error")
	}
}

func TestSubagentTimeout(t *testing.T) {
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM: &blockingSubagentModel{},
		Timeout: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	if _, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p")); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestSubagentResultTruncation(t *testing.T) {
	model := newFakeSubagentModel(fakeSubagentResponse{msg: subagentAssistantMsg(strings.Repeat("x", 100))})
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM:        model,
		MaxResultChars: 100,
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	got, err := tl.InvokableRun(context.Background(), subagentArgs(t, "n", "t", "p"))
	if err != nil {
		t.Fatalf("invokable run: %v", err)
	}
	if !strings.Contains(got, "truncated") {
		t.Fatalf("result not truncated: %q", got)
	}
	if !strings.HasSuffix(got, "\n</subagent_result>") {
		t.Fatalf("result must stay closed: %q", got)
	}
}

func TestSubagentConcurrent(t *testing.T) {
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM:        &constantSubagentModel{content: "ok"},
		MaxConcurrency: 2,
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			got, err := tl.InvokableRun(context.Background(), subagentArgs(t, fmt.Sprintf("n%d", i), "t", "p"))
			if err != nil {
				t.Errorf("call %d: %v", i, err)
				return
			}
			if !strings.Contains(got, "ok") {
				t.Errorf("call %d result: %q", i, got)
			}
		})
	}
	wg.Wait()
}

func TestBuildSubagentSystemPrompt(t *testing.T) {
	got := buildSubagentSystemPrompt("base", "name", "title")
	if !strings.Contains(got, "base") || !strings.Contains(got, "Your name: name") || !strings.Contains(got, "Your task: title") {
		t.Fatalf("prompt: %q", got)
	}
}

func TestFormatSubagentResult(t *testing.T) {
	got := formatSubagentResult("n", "t", "content", 0)
	if got != "<subagent_result name=\"n\" title=\"t\">\ncontent\n</subagent_result>" {
		t.Fatalf("result: %q", got)
	}

	got = formatSubagentResult("n", "t", strings.Repeat("x", 100), 100)
	if !utf8.ValidString(got) {
		t.Fatalf("truncated result is not valid utf8: %q", got)
	}
	if n := utf8.RuneCountInString(got); n != 100 {
		t.Fatalf("truncated result runes = %d, want 100: %q", n, got)
	}
	if !strings.HasSuffix(got, "\n</subagent_result>") {
		t.Fatalf("closing tag must stay intact: %q", got)
	}
	if !strings.Contains(got, subagentResultTruncatedMark) {
		t.Fatalf("truncated result: %q", got)
	}
	if !strings.HasPrefix(got, "<subagent_result name=\"n\" title=\"t\">\n") {
		t.Fatalf("opening tag must stay intact: %q", got)
	}
	if strings.Count(got, "</subagent_result>") != 1 {
		t.Fatalf("unexpected closing tag count: %q", got)
	}

	got = formatSubagentResult("n", "t", strings.Repeat("x", 100), 20)
	if want := formatSubagentResult("n", "t", "", 0); got != want {
		t.Fatalf("budget too small result: %q, want %q", got, want)
	}
}

func TestSubagentSemaphoreAcquireError(t *testing.T) {
	tl, err := NewSubagentTool(context.Background(), SubagentConfig{
		BaseLLM: newFakeSubagentModel(),
	})
	if err != nil {
		t.Fatalf("new subagent tool: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := tl.InvokableRun(ctx, subagentArgs(t, "n", "t", "p")); err == nil {
		t.Fatal("expected acquire error")
	}
}
