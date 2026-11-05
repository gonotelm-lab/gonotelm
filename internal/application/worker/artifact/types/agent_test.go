package types

import (
	"context"
	"sync"
	"testing"

	agentv2 "github.com/gonotelm-lab/gonotelm/pkg/agent/v2"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type artifactModelCall struct {
	messages []*schema.Message
	tools    []string
}

type artifactModelState struct {
	mu    sync.Mutex
	calls []artifactModelCall
}

type artifactModel struct {
	state *artifactModelState
	msg   *schema.Message
	tools []*schema.ToolInfo
}

var _ model.ToolCallingChatModel = (*artifactModel)(nil)

func newArtifactModel(msg *schema.Message) *artifactModel {
	return &artifactModel{state: &artifactModelState{}, msg: msg}
}

func (m *artifactModel) record(in []*schema.Message) {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	tools := make([]string, 0, len(m.tools))
	for _, info := range m.tools {
		tools = append(tools, info.Name)
	}
	m.state.calls = append(m.state.calls, artifactModelCall{
		messages: append([]*schema.Message(nil), in...),
		tools:    tools,
	})
}

func (m *artifactModel) Generate(
	ctx context.Context,
	in []*schema.Message,
	opts ...model.Option,
) (*schema.Message, error) {
	m.record(in)
	return m.msg, nil
}

func (m *artifactModel) Stream(
	ctx context.Context,
	in []*schema.Message,
	opts ...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	m.record(in)
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(m.msg, nil)
	}()
	return reader, nil
}

func (m *artifactModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return &artifactModel{state: m.state, msg: m.msg, tools: tools}, nil
}

func (m *artifactModel) snapshotCalls() []artifactModelCall {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()
	out := make([]artifactModelCall, len(m.state.calls))
	copy(out, m.state.calls)
	return out
}

type artifactTool struct {
	name string
	run  func(context.Context, string) (string, error)
}

var _ tool.InvokableTool = (*artifactTool)(nil)

func (t *artifactTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name, Desc: t.name}, nil
}

func (t *artifactTool) InvokableRun(
	ctx context.Context,
	args string,
	opts ...tool.Option,
) (string, error) {
	if t.run == nil {
		return "ok", nil
	}
	return t.run(ctx, args)
}

func artifactAssistantMsg(content string) *schema.Message {
	msg := schema.AssistantMessage(content, nil)
	msg.ResponseMeta = &schema.ResponseMeta{
		FinishReason: "stop",
		Usage:        &schema.TokenUsage{PromptTokens: 2, CompletionTokens: 1, TotalTokens: 3},
	}
	return msg
}

func newTestArtifactAgent(t *testing.T, m *artifactModel, tools map[string]tool.InvokableTool) *Agent {
	t.Helper()
	cfg := agentv2.RunnerConfig{
		Name:  "artifact-test",
		Model: m,
		Tools: toolSlice(tools),
	}
	runner, err := agentv2.NewRunner[*SessionState](context.Background(), cfg)
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	return &Agent{
		cfg:     cfg,
		tools:   tools,
		runner:  runner,
		session: agentv2.NewSession(&SessionState{}),
	}
}

func TestAgentReactAndTokenUsage(t *testing.T) {
	model := newArtifactModel(artifactAssistantMsg("ok"))
	tools := map[string]tool.InvokableTool{"echo": &artifactTool{name: "echo"}}
	agent := newTestArtifactAgent(t, model, tools)

	res, err := agent.React(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("react: %v", err)
	}
	if res.Content != "ok" {
		t.Fatalf("final: %+v", res)
	}
	if got := agent.TokenUsage().TotalTokens; got != 3 {
		t.Fatalf("usage: %d", got)
	}

	calls := model.snapshotCalls()
	if len(calls) != 1 || len(calls[0].tools) != 1 || calls[0].tools[0] != "echo" {
		t.Fatalf("model calls: %+v", calls)
	}
}

func TestAgentAppendTools(t *testing.T) {
	model := newArtifactModel(artifactAssistantMsg("ok"))
	agent := newTestArtifactAgent(t, model, map[string]tool.InvokableTool{
		"a": &artifactTool{name: "a"},
	})

	if err := agent.AppendTools(map[string]tool.InvokableTool{
		"b": &artifactTool{name: "b"},
	}); err != nil {
		t.Fatalf("append tools: %v", err)
	}
	if err := agent.AppendTools(nil); err != nil {
		t.Fatalf("append empty tools: %v", err)
	}

	if _, err := agent.React(context.Background(), []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("react: %v", err)
	}
	calls := model.snapshotCalls()
	if len(calls) != 1 || len(calls[0].tools) != 2 {
		t.Fatalf("model calls: %+v", calls)
	}
	if calls[0].tools[0] != "a" || calls[0].tools[1] != "b" {
		t.Fatalf("tools: %v", calls[0].tools)
	}
}

func TestAgentBaseLLM(t *testing.T) {
	model := newArtifactModel(artifactAssistantMsg("ok"))
	agent := newTestArtifactAgent(t, model, nil)

	if agent.BaseLLM() != model {
		t.Fatal("BaseLLM should return the configured base model")
	}
}

func TestAgentReactStream(t *testing.T) {
	model := newArtifactModel(artifactAssistantMsg("streamed"))
	agent := newTestArtifactAgent(t, model, nil)

	res, err := agent.ReactStream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("react stream: %v", err)
	}
	if res.Content != "streamed" {
		t.Fatalf("final: %+v", res)
	}
}

func TestAgentAppendToolsOverride(t *testing.T) {
	model := newArtifactModel(artifactAssistantMsg("ok"))
	agent := newTestArtifactAgent(t, model, map[string]tool.InvokableTool{
		"a": &artifactTool{name: "a"},
	})

	if err := agent.AppendTools(map[string]tool.InvokableTool{
		"a": &artifactTool{name: "a"},
	}); err != nil {
		t.Fatalf("append duplicate tool: %v", err)
	}

	if _, err := agent.React(context.Background(), []*schema.Message{schema.UserMessage("hi")}); err != nil {
		t.Fatalf("react: %v", err)
	}
	calls := model.snapshotCalls()
	if len(calls) != 1 || len(calls[0].tools) != 1 || calls[0].tools[0] != "a" {
		t.Fatalf("model calls: %+v", calls)
	}
}

func TestToolSlice(t *testing.T) {
	got := toolSlice(map[string]tool.InvokableTool{
		"a": &artifactTool{name: "a"},
		"b": &artifactTool{name: "b"},
	})
	if len(got) != 2 {
		t.Fatalf("tool slice: %v", got)
	}
	for _, item := range got {
		if item == nil {
			t.Fatal("nil tool in slice")
		}
	}
}
