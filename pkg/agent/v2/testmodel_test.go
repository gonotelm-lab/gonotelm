package agent

import (
	"context"
	"fmt"
	"sync"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type scriptedResponse struct {
	msg    *schema.Message
	chunks []*schema.Message
	err    error
}

type modelCall struct {
	messages []*schema.Message
	tools    []string
}

type scriptState struct {
	mu        sync.Mutex
	responses []scriptedResponse
	index     int
	calls     []modelCall
}

type scriptedModel struct {
	state *scriptState
	tools []*schema.ToolInfo
}

var _ einomodel.ToolCallingChatModel = (*scriptedModel)(nil)

func newScriptedModel(responses ...scriptedResponse) *scriptedModel {
	return &scriptedModel{state: &scriptState{responses: responses}}
}

func (m *scriptedModel) Generate(
	ctx context.Context,
	in []*schema.Message,
	opts ...einomodel.Option,
) (*schema.Message, error) {
	resp := m.nextResponse(in)
	return resp.msg, resp.err
}

func (m *scriptedModel) Stream(
	ctx context.Context,
	in []*schema.Message,
	opts ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	resp := m.nextResponse(in)
	if resp.err != nil {
		return nil, resp.err
	}

	chunks := resp.chunks
	if len(chunks) == 0 && resp.msg != nil {
		chunks = []*schema.Message{resp.msg}
	}

	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		for _, chunk := range chunks {
			if writer.Send(chunk, nil) {
				return
			}
		}
	}()

	return reader, nil
}

func (m *scriptedModel) WithTools(tools []*schema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return &scriptedModel{state: m.state, tools: tools}, nil
}

func (m *scriptedModel) nextResponse(in []*schema.Message) scriptedResponse {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	tools := make([]string, 0, len(m.tools))
	for _, info := range m.tools {
		tools = append(tools, info.Name)
	}
	m.state.calls = append(m.state.calls, modelCall{
		messages: append([]*schema.Message(nil), in...),
		tools:    tools,
	})

	if m.state.index >= len(m.state.responses) {
		return scriptedResponse{err: fmt.Errorf("unexpected model call #%d", m.state.index)}
	}

	resp := m.state.responses[m.state.index]
	m.state.index++
	return resp
}

func (m *scriptedModel) snapshotCalls() []modelCall {
	m.state.mu.Lock()
	defer m.state.mu.Unlock()

	out := make([]modelCall, len(m.state.calls))
	copy(out, m.state.calls)
	return out
}

type fakeTool struct {
	name    string
	infoErr error
	nilInfo bool
	run     func(ctx context.Context, args string) (string, error)
}

var _ einotool.InvokableTool = (*fakeTool)(nil)

func (t *fakeTool) Info(context.Context) (*schema.ToolInfo, error) {
	if t.infoErr != nil {
		return nil, t.infoErr
	}
	if t.nilInfo {
		return nil, nil
	}
	return &schema.ToolInfo{Name: t.name, Desc: t.name}, nil
}

func (t *fakeTool) InvokableRun(ctx context.Context, args string, opts ...einotool.Option) (string, error) {
	if t.run == nil {
		return "ok", nil
	}
	return t.run(ctx, args)
}

func assistantMsg(content string, usage *schema.TokenUsage) *schema.Message {
	msg := schema.AssistantMessage(content, nil)
	msg.ResponseMeta = &schema.ResponseMeta{FinishReason: "stop", Usage: usage}
	return msg
}

func toolCallAssistant(content string, calls ...schema.ToolCall) *schema.Message {
	msg := schema.AssistantMessage(content, calls)
	msg.ResponseMeta = &schema.ResponseMeta{FinishReason: "tool_calls"}
	return msg
}

func newToolCall(id, name, args string) schema.ToolCall {
	return schema.ToolCall{
		ID:   id,
		Type: "function",
		Function: schema.FunctionCall{
			Name:      name,
			Arguments: args,
		},
	}
}

func tokenUsage(prompt, completion int) *schema.TokenUsage {
	return &schema.TokenUsage{
		PromptTokens:     prompt,
		CompletionTokens: completion,
		TotalTokens:      prompt + completion,
	}
}

func partialToolCall(id string, index int, name, args, finishReason string) *schema.Message {
	idx := index
	msg := &schema.Message{
		Role: schema.Assistant,
		ToolCalls: []schema.ToolCall{{
			Index: &idx,
			ID:    id,
			Type:  "function",
			Function: schema.FunctionCall{
				Name:      name,
				Arguments: args,
			},
		}},
	}
	if finishReason != "" {
		msg.ResponseMeta = &schema.ResponseMeta{FinishReason: finishReason}
	}
	return msg
}

type withToolsErrorModel struct {
	*scriptedModel
}

func (m *withToolsErrorModel) WithTools(
	[]*schema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return nil, fmt.Errorf("bind failed")
}

type nilStreamModel struct {
	*scriptedModel
}

func (m *nilStreamModel) Stream(
	context.Context, []*schema.Message, ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	return nil, nil
}

type recvErrorModel struct{}

var _ einomodel.ToolCallingChatModel = (*recvErrorModel)(nil)

func (m *recvErrorModel) Generate(
	context.Context, []*schema.Message, ...einomodel.Option,
) (*schema.Message, error) {
	return assistantMsg("ok", nil), nil
}

func (m *recvErrorModel) Stream(
	context.Context, []*schema.Message, ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(nil, fmt.Errorf("recv boom"))
	}()
	return reader, nil
}

func (m *recvErrorModel) WithTools(
	[]*schema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

type cancelOnDeltaModel struct {
	cancel context.CancelFunc
}

var _ einomodel.ToolCallingChatModel = (*cancelOnDeltaModel)(nil)

func (m *cancelOnDeltaModel) Generate(
	context.Context, []*schema.Message, ...einomodel.Option,
) (*schema.Message, error) {
	return assistantMsg("ok", nil), nil
}

func (m *cancelOnDeltaModel) Stream(
	ctx context.Context, _ []*schema.Message, _ ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(&schema.Message{Role: schema.Assistant, Content: "a"}, nil)
		<-ctx.Done()
	}()
	return reader, nil
}

func (m *cancelOnDeltaModel) WithTools(
	[]*schema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}
