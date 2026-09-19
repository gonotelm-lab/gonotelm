package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pkgllm "github.com/gonotelm-lab/gonotelm/pkg/llm"

	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ---------- fakes ----------

type fakeChatModel struct {
	mu      sync.Mutex
	replies []*schema.Message
	idx     int

	toolNames     [][]string // tool names passed to WithTools, one entry per call
	systemPrompts []string   // system message content seen on each Generate
	subagentFlags []bool     // context subagent flag seen on each Generate

	beforeGenerate func()
}

func (m *fakeChatModel) Generate(
	ctx context.Context,
	input []*schema.Message,
	_ ...model.Option,
) (*schema.Message, error) {
	if m.beforeGenerate != nil {
		m.beforeGenerate()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.subagentFlags = append(m.subagentFlags, pkgcontext.IsSubagent(ctx))

	for _, msg := range input {
		if msg.Role == schema.System {
			m.systemPrompts = append(m.systemPrompts, msg.Content)
			break
		}
	}

	if m.idx >= len(m.replies) {
		return nil, fmt.Errorf("fake chat model: no more replies")
	}
	msg := m.replies[m.idx]
	m.idx++
	return msg, nil
}

func (m *fakeChatModel) Stream(
	context.Context,
	[]*schema.Message,
	...model.Option,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, fmt.Errorf("fake chat model: stream not implemented")
}

func (m *fakeChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	names := make([]string, 0, len(tools))
	for _, info := range tools {
		names = append(names, info.Name)
	}

	m.mu.Lock()
	m.toolNames = append(m.toolNames, names)
	m.mu.Unlock()
	return m, nil
}

func (m *fakeChatModel) recordedToolNames() [][]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([][]string, len(m.toolNames))
	copy(out, m.toolNames)
	return out
}

func (m *fakeChatModel) recordedSystemPrompts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.systemPrompts))
	copy(out, m.systemPrompts)
	return out
}

func (m *fakeChatModel) recordedSubagentFlags() []bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]bool, len(m.subagentFlags))
	copy(out, m.subagentFlags)
	return out
}

type fakeTool struct {
	name  string
	calls atomic.Int64
	// 记录调用时 ctx 是否带 subagent 标记
	subagentCtx atomic.Bool
}

func (t *fakeTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        t.name,
		Desc:        t.name,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (t *fakeTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
	t.calls.Add(1)
	if pkgcontext.IsSubagent(ctx) {
		t.subagentCtx.Store(true)
	}
	return "tool-ok", nil
}

func stopMessage(content string) *schema.Message {
	return &schema.Message{
		Role:    schema.Assistant,
		Content: content,
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: pkgllm.FinishReasonStop,
			Usage:        &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
	}
}

func toolCallMessage(toolName, args string) *schema.Message {
	return &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			ID:       "call-1",
			Type:     "function",
			Function: schema.FunctionCall{Name: toolName, Arguments: args},
		}},
		ResponseMeta: &schema.ResponseMeta{
			FinishReason: pkgllm.FinishReasonToolCalls,
			Usage:        &schema.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		},
	}
}

// ---------- helpers ----------

func newFakeModel(replies ...*schema.Message) *fakeChatModel {
	return &fakeChatModel{replies: replies}
}

func singleTool() map[string]tool.InvokableTool {
	return map[string]tool.InvokableTool{"T": &fakeTool{name: "T"}}
}

// testConfig 返回一个最小可用的 spawn 配置 模型 + 工具集 没有角色/描述
func testConfig(llm model.ToolCallingChatModel, tools map[string]tool.InvokableTool) SubagentConfig {
	return SubagentConfig{
		BaseLLM: llm,
		Tools:   tools,
	}
}

func mustNewTool(t *testing.T, cfg SubagentConfig) *SubagentTool {
	t.Helper()
	subagentTool, err := NewSubagentTool(cfg)
	if err != nil {
		t.Fatalf("NewSubagentTool: %v", err)
	}
	return subagentTool
}

// ---------- construction validation ----------

func TestNewSubagentToolValidation(t *testing.T) {
	cases := []struct {
		name string
		cfg  SubagentConfig
	}{
		{
			name: "nil base llm",
			cfg: SubagentConfig{
				Tools: singleTool(),
			},
		},
		{
			name: "nil tool",
			cfg: SubagentConfig{
				BaseLLM: newFakeModel(stopMessage("ok")),
				Tools:   map[string]tool.InvokableTool{"T": nil},
			},
		},
		{
			name: "tool key mismatch",
			cfg: SubagentConfig{
				BaseLLM: newFakeModel(stopMessage("ok")),
				Tools:   map[string]tool.InvokableTool{"Wrong": &fakeTool{name: "Right"}},
			},
		},
		{
			name: "declares subagent tool",
			cfg: SubagentConfig{
				BaseLLM: newFakeModel(stopMessage("ok")),
				Tools: map[string]tool.InvokableTool{
					SubagentToolName: &fakeTool{name: SubagentToolName},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSubagentTool(tc.cfg); err == nil {
				t.Fatalf("expected error, got nil")
			}
		})
	}
}

func TestNewSubagentToolDefaults(t *testing.T) {
	subagentTool := mustNewTool(t, SubagentConfig{
		BaseLLM: newFakeModel(stopMessage("ok")),
		Tools:   singleTool(),
	})

	if subagentTool.maxRound != defaultSubagentMaxRound {
		t.Fatalf("maxRound = %d, want %d", subagentTool.maxRound, defaultSubagentMaxRound)
	}
	if subagentTool.maxResultChars != defaultSubagentResultChars {
		t.Fatalf("maxResultChars = %d, want %d", subagentTool.maxResultChars, defaultSubagentResultChars)
	}
	if subagentTool.systemPrompt != defaultSubagentSystemPrompt {
		t.Fatalf("systemPrompt = %q, want default", subagentTool.systemPrompt)
	}
}

func TestNewSubagentToolKeepsExplicitConfig(t *testing.T) {
	subagentTool := mustNewTool(t, SubagentConfig{
		BaseLLM:        newFakeModel(stopMessage("ok")),
		Tools:          singleTool(),
		SystemPrompt:   "custom",
		MaxRound:       7,
		MaxResultChars: 123,
	})

	if subagentTool.maxRound != 7 {
		t.Fatalf("maxRound = %d, want 7", subagentTool.maxRound)
	}
	if subagentTool.maxResultChars != 123 {
		t.Fatalf("maxResultChars = %d, want 123", subagentTool.maxResultChars)
	}
	if subagentTool.systemPrompt != "custom" {
		t.Fatalf("systemPrompt = %q, want custom", subagentTool.systemPrompt)
	}
}

// ---------- tool info / schema ----------

func TestSubagentToolInfo(t *testing.T) {
	subagentTool := mustNewTool(t, testConfig(newFakeModel(stopMessage("ok")), singleTool()))

	info, err := subagentTool.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != SubagentToolName {
		t.Fatalf("Info.Name = %q, want %q", info.Name, SubagentToolName)
	}
	if !strings.Contains(info.Desc, "isolated context") {
		t.Fatalf("Info.Desc should describe the spawn/context-isolation capability: %q", info.Desc)
	}

	// 入参只有标题和任务正文 没有 description / profile 之类的路由参数
	properties := subagentSchemaProperties(t, info)
	for _, want := range []string{"name", "title", "prompt"} {
		if _, ok := properties[want]; !ok {
			t.Fatalf("schema missing %q property: %v", want, properties)
		}
	}
	for _, unwanted := range []string{"description", "profile"} {
		if _, ok := properties[unwanted]; ok {
			t.Fatalf("schema should not expose %q as an input property", unwanted)
		}
	}
}

func subagentSchemaProperties(t *testing.T, info *schema.ToolInfo) map[string]json.RawMessage {
	t.Helper()

	jsonSchema, err := info.ToJSONSchema()
	if err != nil {
		t.Fatalf("ToJSONSchema: %v", err)
	}

	raw, err := json.Marshal(jsonSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}

	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	return doc.Properties
}

// ---------- run ----------

func TestSubagentToolRun(t *testing.T) {
	llm := newFakeModel(
		toolCallMessage("T", `{}`),
		stopMessage("subagent says hi"),
	)
	declaredTool := &fakeTool{name: "T"}

	subagentTool := mustNewTool(t, testConfig(llm, map[string]tool.InvokableTool{"T": declaredTool}))

	out, err := subagentTool.InvokableRun(context.Background(), `{"name":"scout","title":"probe","prompt":"do it"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	if !strings.Contains(out, `<subagent_result name="scout" title="probe">`) {
		t.Fatalf("result missing opening tag: %q", out)
	}
	if !strings.Contains(out, "subagent says hi") {
		t.Fatalf("result missing content: %q", out)
	}
	if !strings.HasSuffix(out, "</subagent_result>") {
		t.Fatalf("result missing closing tag: %q", out)
	}

	if got := declaredTool.calls.Load(); got != 1 {
		t.Fatalf("subagent tool calls = %d, want 1", got)
	}
	if got := llm.recordedToolNames(); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "T" {
		t.Fatalf("unexpected WithTools calls: %v", got)
	}
}

func TestSubagentToolSystemPromptCarriesNameAndTitle(t *testing.T) {
	const input = `{"name":"scout","title":"probe the sources","prompt":"do it"}`

	t.Run("default system prompt", func(t *testing.T) {
		llm := newFakeModel(stopMessage("ok"))
		subagentTool := mustNewTool(t, testConfig(llm, singleTool()))

		if _, err := subagentTool.InvokableRun(context.Background(), input); err != nil {
			t.Fatalf("InvokableRun: %v", err)
		}

		prompts := llm.recordedSystemPrompts()
		if len(prompts) != 1 {
			t.Fatalf("system prompts = %d, want 1", len(prompts))
		}
		for _, want := range []string{defaultSubagentSystemPrompt, "scout", "probe the sources"} {
			if !strings.Contains(prompts[0], want) {
				t.Fatalf("system prompt missing %q: %q", want, prompts[0])
			}
		}
	})

	t.Run("custom system prompt", func(t *testing.T) {
		llm := newFakeModel(stopMessage("ok"))
		cfg := testConfig(llm, singleTool())
		cfg.SystemPrompt = "You are a careful researcher."
		subagentTool := mustNewTool(t, cfg)

		if _, err := subagentTool.InvokableRun(context.Background(), input); err != nil {
			t.Fatalf("InvokableRun: %v", err)
		}

		prompts := llm.recordedSystemPrompts()
		if len(prompts) != 1 {
			t.Fatalf("system prompts = %d, want 1", len(prompts))
		}
		for _, want := range []string{"You are a careful researcher.", "scout", "probe the sources"} {
			if !strings.Contains(prompts[0], want) {
				t.Fatalf("system prompt missing %q: %q", want, prompts[0])
			}
		}
	})
}

// TestSubagentToolMarksSubagentContext 验证子代理链路上的 llm 调用与工具调用都能
// 从 ctx 读到 subagent 标记，且父 ctx 不被污染（日志与 llm record 依赖该标记）
func TestSubagentToolMarksSubagentContext(t *testing.T) {
	llm := newFakeModel(
		toolCallMessage("T", `{}`),
		stopMessage("ok"),
	)
	declaredTool := &fakeTool{name: "T"}
	subagentTool := mustNewTool(t, testConfig(llm, map[string]tool.InvokableTool{"T": declaredTool}))

	parentCtx := context.Background()
	if _, err := subagentTool.InvokableRun(parentCtx, `{"name":"n","title":"t","prompt":"p"}`); err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	flags := llm.recordedSubagentFlags()
	if len(flags) == 0 {
		t.Fatal("expected subagent llm calls")
	}
	for i, flag := range flags {
		if !flag {
			t.Fatalf("llm call %d not marked as subagent", i)
		}
	}

	if !declaredTool.subagentCtx.Load() {
		t.Fatal("subagent tool call not marked as subagent")
	}

	if pkgcontext.IsSubagent(parentCtx) {
		t.Fatal("parent context must not be marked as subagent")
	}
}

// TestSubagentToolToolsSurviveAcrossRuns 回归 NewFinalRoundHook -> StripTools() -> clear(map)
// 会把子代理声明的工具集清空的问题 每次 Run 必须绑定声明 map 的副本
func TestSubagentToolToolsSurviveAcrossRuns(t *testing.T) {
	llm := newFakeModel(
		toolCallMessage("T", `{}`), stopMessage("first"),
		toolCallMessage("T", `{}`), stopMessage("second"),
	)
	declaredTool := &fakeTool{name: "T"}

	subagentTool := mustNewTool(t, testConfig(llm, map[string]tool.InvokableTool{"T": declaredTool}))

	for i := range 2 {
		out, err := subagentTool.InvokableRun(context.Background(), `{"name":"scout","title":"probe","prompt":"do it"}`)
		if err != nil {
			t.Fatalf("run %d: %v", i, err)
		}
		if !strings.Contains(out, []string{"first", "second"}[i]) {
			t.Fatalf("run %d unexpected result: %q", i, out)
		}
	}

	if got := len(subagentTool.tools); got != 1 {
		t.Fatalf("declared tools were mutated: len = %d, want 1", got)
	}
	if got := llm.recordedToolNames(); len(got) != 2 {
		t.Fatalf("WithTools called %d times, want 2 (tools lost after first run): %v", len(got), got)
	}
	if got := declaredTool.calls.Load(); got != 2 {
		t.Fatalf("subagent tool calls = %d, want 2", got)
	}
}

func TestSubagentToolInputErrors(t *testing.T) {
	subagentTool := mustNewTool(t, testConfig(newFakeModel(stopMessage("a")), singleTool()))

	t.Run("empty name", func(t *testing.T) {
		if _, err := subagentTool.InvokableRun(context.Background(), `{"name":"   ","title":"t","prompt":"p"}`); err == nil {
			t.Fatal("expected error for empty name")
		}
	})

	t.Run("empty title", func(t *testing.T) {
		if _, err := subagentTool.InvokableRun(context.Background(), `{"name":"n","title":"   ","prompt":"p"}`); err == nil {
			t.Fatal("expected error for empty title")
		}
	})

	t.Run("empty prompt", func(t *testing.T) {
		if _, err := subagentTool.InvokableRun(context.Background(), `{"name":"n","title":"t","prompt":"   "}`); err == nil {
			t.Fatal("expected error for empty prompt")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		if _, err := subagentTool.InvokableRun(context.Background(), `not-json`); err == nil {
			t.Fatal("expected error for invalid json")
		}
	})
}

func TestSubagentToolMaxCallsPerRun(t *testing.T) {
	cfg := testConfig(newFakeModel(stopMessage("a"), stopMessage("b")), singleTool())
	cfg.MaxCallsPerRun = 1
	subagentTool := mustNewTool(t, cfg)

	if _, err := subagentTool.InvokableRun(context.Background(), `{"name":"n","title":"t","prompt":"p"}`); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := subagentTool.InvokableRun(context.Background(), `{"name":"n","title":"t","prompt":"p"}`); err == nil {
		t.Fatal("expected call limit error on second call")
	}
}

func TestSubagentToolResultTruncated(t *testing.T) {
	const maxChars = 100

	cfg := testConfig(newFakeModel(stopMessage(strings.Repeat("x", 5000))), singleTool())
	cfg.MaxResultChars = maxChars
	subagentTool := mustNewTool(t, cfg)

	out, err := subagentTool.InvokableRun(context.Background(), `{"name":"n","title":"t","prompt":"p"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}

	const suffix = "\n... [subagent result truncated]"
	if !strings.HasSuffix(out, suffix) {
		t.Fatalf("result not truncated: %q", out)
	}
	if got := len([]rune(out)); got != maxChars+len([]rune(suffix)) {
		t.Fatalf("truncated length = %d, want %d", got, maxChars+len([]rune(suffix)))
	}
}

func TestSubagentToolMaxConcurrency(t *testing.T) {
	var inflight, peak atomic.Int64

	llm := &fakeChatModel{
		replies: []*schema.Message{stopMessage("a"), stopMessage("b")},
		beforeGenerate: func() {
			n := inflight.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			inflight.Add(-1)
		},
	}

	cfg := testConfig(llm, singleTool())
	cfg.MaxConcurrency = 1
	subagentTool := mustNewTool(t, cfg)

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = subagentTool.InvokableRun(ctx, `{"name":"n","title":"t","prompt":"p"}`)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
	}
	if got := peak.Load(); got != 1 {
		t.Fatalf("peak concurrency = %d, want 1", got)
	}
}
