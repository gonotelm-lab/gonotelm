package agent

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	agentv2 "github.com/gonotelm-lab/gonotelm/pkg/agent/v2"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	testDocId      = "0123456789abcdef0123456789abcdef"
	testPhaseArgs  = `{"summary":"检索资料","description":"先读一遍来源","is_final":true}`
	testCiteArgs   = `{"source_doc_ids":["` + testDocId + `"]}`
	testPhase1Body = "阶段一的中间内容"
)

// scriptedModel 按顺序回放预置响应，Generate 与 Stream 共享同一个序列。
type scriptedModel struct {
	mu        sync.Mutex
	responses []scriptedResponse
	index     int
}

type scriptedResponse struct {
	msg    *einoschema.Message
	chunks []*einoschema.Message
	err    error
}

var _ einomodel.ToolCallingChatModel = (*scriptedModel)(nil)

func (m *scriptedModel) next() scriptedResponse {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.index >= len(m.responses) {
		return scriptedResponse{err: fmt.Errorf("unexpected model call #%d", m.index)}
	}

	resp := m.responses[m.index]
	m.index++
	return resp
}

func (m *scriptedModel) calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.index
}

func (m *scriptedModel) Generate(
	context.Context, []*einoschema.Message, ...einomodel.Option,
) (*einoschema.Message, error) {
	resp := m.next()
	return resp.msg, resp.err
}

func (m *scriptedModel) Stream(
	context.Context, []*einoschema.Message, ...einomodel.Option,
) (*einoschema.StreamReader[*einoschema.Message], error) {
	resp := m.next()
	if resp.err != nil {
		return nil, resp.err
	}

	chunks := resp.chunks
	if len(chunks) == 0 && resp.msg != nil {
		chunks = []*einoschema.Message{resp.msg}
	}

	return einoschema.StreamReaderFromArray(chunks), nil
}

func (m *scriptedModel) WithTools([]*einoschema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

type recorder struct {
	mu sync.Mutex

	content       []string
	reasoning     []string
	responseStart int
	responseEnd   int
	thinkStart    int
	phases        []Phase
	rounds        [][]*einoschema.Message
}

func (r *recorder) hooks() *Hooks {
	return &Hooks{
		ThinkStart: func(context.Context) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.thinkStart++
		},
		ThinkDelta: func(_ context.Context, delta string) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.reasoning = append(r.reasoning, delta)
		},
		ResponseStart: func(context.Context) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.responseStart++
		},
		ResponseDelta: func(_ context.Context, delta string) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.content = append(r.content, delta)
		},
		ResponseEnd: func(context.Context) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.responseEnd++
		},
		PhaseMarkHook: func(_ context.Context, phase Phase) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.phases = append(r.phases, phase)
		},
		RoundFinishedHook: func(_ context.Context, msgs []*einoschema.Message) {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.rounds = append(r.rounds, msgs)
		},
	}
}

func (r *recorder) snapshot() recorder {
	r.mu.Lock()
	defer r.mu.Unlock()
	return recorder{
		content:       append([]string(nil), r.content...),
		reasoning:     append([]string(nil), r.reasoning...),
		responseStart: r.responseStart,
		responseEnd:   r.responseEnd,
		thinkStart:    r.thinkStart,
		phases:        append([]Phase(nil), r.phases...),
		rounds:        append([][]*einoschema.Message(nil), r.rounds...),
	}
}

func toolCallMessage(content string, id, name, args string) *einoschema.Message {
	msg := einoschema.AssistantMessage(content, []einoschema.ToolCall{{
		ID:   id,
		Type: "function",
		Function: einoschema.FunctionCall{
			Name:      name,
			Arguments: args,
		},
	}})
	msg.ResponseMeta = &einoschema.ResponseMeta{FinishReason: "tool_calls"}
	return msg
}

func assistantMessage(content, reasoning string) *einoschema.Message {
	msg := einoschema.AssistantMessage(content, nil)
	msg.ReasoningContent = reasoning
	msg.ResponseMeta = &einoschema.ResponseMeta{FinishReason: "stop"}
	return msg
}

func newTestRunner(
	t *testing.T,
	model *scriptedModel,
	bound []agentv2.Tool,
) *agentv2.Runner[*SessionState] {
	t.Helper()

	runner, err := agentv2.NewRunner[*SessionState](context.Background(), agentv2.RunnerConfig{
		Name:     "chat-test",
		Model:    model,
		Tools:    bound,
		MaxRound: 6,
	})
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}

	return runner
}

// TestRunChatTwoPhases 覆盖「阶段一非流式探索 → 阶段二流式输出」的完整链路：
// 阶段一的内容与推理不下发，阶段标记与引用登记生效，阶段二只下发增量。
func TestRunChatTwoPhases(t *testing.T) {
	model := &scriptedModel{responses: []scriptedResponse{
		{msg: toolCallMessage(testPhase1Body, "call-1", tools.MarkPhaseToolName, testPhaseArgs)},
		{msg: toolCallMessage(testPhase1Body, "call-2", tools.CiteSourceDocToolName, testCiteArgs)},
		{chunks: []*einoschema.Message{
			{Role: einoschema.Assistant, ReasoningContent: "想一下"},
			{Role: einoschema.Assistant, Content: "最终"},
			{Role: einoschema.Assistant, Content: "回答"},
		}},
	}}

	state := &SessionState{curRunPhase: runPhase1}
	runner := newTestRunner(t, model, []agentv2.Tool{
		tools.NewMarkPhaseTool(),
		tools.NewCiteSourceDocTool(
			tools.SourceDocPermissionCheckerFunc(func(context.Context, []valobj.Id) error { return nil }),
			composeCitationCollector(state),
		),
	})
	session := agentv2.NewSession(state)
	session.Append(einoschema.UserMessage("介绍一下这本笔记"))

	rec := &recorder{}
	res, err := runChat(context.Background(), runner, session, rec.hooks())
	if err != nil {
		t.Fatalf("run chat: %v", err)
	}

	if res.FinalMessage == nil || res.FinalMessage.Content != "最终回答" {
		t.Fatalf("unexpected final message: %+v", res.FinalMessage)
	}

	citations := res.SourceDocCitations
	if len(citations) != 1 || citations[0].String() != testDocId {
		t.Fatalf("unexpected citations: %v", citations)
	}

	got := rec.snapshot()

	// 阶段一的正文与推理都不下发，只有阶段二的流式增量可见。
	if content := strings.Join(got.content, ""); content != "最终回答" {
		t.Fatalf("unexpected pushed content: %q", content)
	}
	if strings.Join(got.reasoning, "") != "想一下" {
		t.Fatalf("unexpected pushed reasoning: %v", got.reasoning)
	}
	if got.responseStart != 1 || got.responseEnd != 1 {
		t.Fatalf("unexpected response boundary calls: start=%d end=%d", got.responseStart, got.responseEnd)
	}
	if got.thinkStart != 1 {
		t.Fatalf("unexpected think start calls: %d", got.thinkStart)
	}

	// 阶段标记只回调一次，且阶段一结束后不会被阶段二重置。
	if len(got.phases) != 1 || !got.phases[0].IsFinal || got.phases[0].Summary != "检索资料" {
		t.Fatalf("unexpected phases: %+v", got.phases)
	}

	// 三轮结果都要落库：阶段一两轮各「assistant + tool」，阶段二一轮 assistant。
	if len(got.rounds) != 3 {
		t.Fatalf("unexpected round finished calls: %d", len(got.rounds))
	}
	if len(got.rounds[0]) != 2 || len(got.rounds[2]) != 1 {
		t.Fatalf("unexpected round messages: %+v", got.rounds)
	}

	// 阶段一的引用登记必须在阶段二开始前完成，阶段二的开场指令也要进入上下文。
	if !state.citationRegistered || state.finalPhaseMarked {
		t.Fatalf("unexpected state: citationRegistered=%v finalPhaseMarked=%v",
			state.citationRegistered, state.finalPhaseMarked)
	}
	if !containsMessage(session.Messages, finalAnswerPrompt) {
		t.Fatalf("final answer prompt is missing from session messages")
	}
	if model.calls() != 3 {
		t.Fatalf("unexpected model calls: %d", model.calls())
	}
}

// TestRunChatManualPushOnPhase1Stop 覆盖阶段一直接 finish_reason=stop 的兼容分支：
// 不再进入阶段二，由 manualPushFinalMsg 手动推送一次完整回答。
func TestRunChatManualPushOnPhase1Stop(t *testing.T) {
	model := &scriptedModel{responses: []scriptedResponse{
		{msg: assistantMessage("直接给出的答案", "")},
	}}

	state := &SessionState{curRunPhase: runPhase1}
	runner := newTestRunner(t, model, nil)
	session := agentv2.NewSession(state)
	session.Append(einoschema.UserMessage("介绍一下这本笔记"))

	rec := &recorder{}
	res, err := runChat(context.Background(), runner, session, rec.hooks())
	if err != nil {
		t.Fatalf("run chat: %v", err)
	}

	if res.FinalMessage == nil || res.FinalMessage.Content != "直接给出的答案" {
		t.Fatalf("unexpected final message: %+v", res.FinalMessage)
	}

	got := rec.snapshot()
	if strings.Join(got.content, "") != "直接给出的答案" {
		t.Fatalf("unexpected pushed content: %v", got.content)
	}
	if got.responseStart != 1 || got.responseEnd != 1 {
		t.Fatalf("unexpected response boundary calls: start=%d end=%d", got.responseStart, got.responseEnd)
	}
	if model.calls() != 1 {
		t.Fatalf("unexpected model calls: %d", model.calls())
	}
	if containsMessage(session.Messages, finalAnswerPrompt) {
		t.Fatalf("phase 2 must be skipped when phase 1 already answered")
	}
}

func containsMessage(msgs []*einoschema.Message, content string) bool {
	for _, msg := range msgs {
		if msg != nil && msg.Content == content {
			return true
		}
	}
	return false
}

func TestCitationCollectorWritesState(t *testing.T) {
	state := &SessionState{}
	docId, err := valobj.NewIdFromString(testDocId)
	if err != nil {
		t.Fatalf("parse doc id: %v", err)
	}

	composeCitationCollector(state).Set([]valobj.Id{docId})

	if !state.citationRegistered {
		t.Fatalf("citationRegistered is not set")
	}
	if len(state.sourceDocCitations) != 1 || state.sourceDocCitations[0] != docId {
		t.Fatalf("unexpected citations: %v", state.sourceDocCitations)
	}
}
