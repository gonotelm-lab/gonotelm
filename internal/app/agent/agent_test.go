package agent

import (
	"context"
	"fmt"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	. "github.com/smartystreets/goconvey/convey"
)

type fakeRound struct {
	assertInput func(msgs []*einoschema.Message)
	output      []*einoschema.Message
}

type fakeToolCallingModel struct {
	rounds []fakeRound
	calls  int
}

var _ einomodel.ToolCallingChatModel = (*fakeToolCallingModel)(nil)

func (f *fakeToolCallingModel) Generate(
	_ context.Context,
	input []*einoschema.Message,
	_ ...einomodel.Option,
) (*einoschema.Message, error) {
	if f.calls >= len(f.rounds) {
		return nil, fmt.Errorf("unexpected generate call #%d", f.calls)
	}

	round := f.rounds[f.calls]
	f.calls++
	if round.assertInput != nil {
		round.assertInput(input)
	}

	if len(round.output) == 0 {
		return nil, fmt.Errorf("empty generate output in round #%d", f.calls-1)
	}

	if len(round.output) == 1 {
		return round.output[0], nil
	}

	concat, err := einoschema.ConcatMessages(round.output)
	if err != nil {
		return nil, fmt.Errorf("concat generate output failed: %w", err)
	}

	return concat, nil
}

func (f *fakeToolCallingModel) Stream(
	_ context.Context,
	input []*einoschema.Message,
	_ ...einomodel.Option,
) (*einoschema.StreamReader[*einoschema.Message], error) {
	if f.calls >= len(f.rounds) {
		return nil, fmt.Errorf("unexpected stream call #%d", f.calls)
	}
	round := f.rounds[f.calls]
	f.calls++
	if round.assertInput != nil {
		round.assertInput(input)
	}
	return einoschema.StreamReaderFromArray(round.output), nil
}

func (f *fakeToolCallingModel) WithTools(
	_ []*einoschema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return f, nil
}

type fakeInvokableTool struct{}

var _ einotool.InvokableTool = (*fakeInvokableTool)(nil)

func (f *fakeInvokableTool) Info(context.Context) (*einoschema.ToolInfo, error) {
	return &einoschema.ToolInfo{Name: "fake_tool"}, nil
}

func (f *fakeInvokableTool) InvokableRun(
	_ context.Context,
	_ string,
	_ ...einotool.Option,
) (string, error) {
	return `{"ok":true}`, nil
}

func TestAgentReactStream_AccumulatesHistoryAcrossToolRounds(t *testing.T) {
	Convey("React 在工具调用轮次后正确累计历史消息", t, func() {
		model := &fakeToolCallingModel{
			rounds: []fakeRound{
				{
					assertInput: func(msgs []*einoschema.Message) {
						So(len(msgs), ShouldEqual, 2)
						So(msgs[0].Role, ShouldEqual, einoschema.System)
						So(msgs[1].Role, ShouldEqual, einoschema.User)
					},
					output: []*einoschema.Message{
						{
							Role: einoschema.Assistant,
							ToolCalls: []einoschema.ToolCall{
								{
									ID:   "call-1",
									Type: "function",
									Function: einoschema.FunctionCall{
										Name:      "fake_tool",
										Arguments: `{"arg":"x"}`,
									},
								},
							},
							ResponseMeta: &einoschema.ResponseMeta{
								FinishReason: llmchat.FinishReasonToolCalls,
							},
						},
					},
				},
				{
					assertInput: func(msgs []*einoschema.Message) {
						So(len(msgs), ShouldEqual, 4)
						So(msgs[0].Role, ShouldEqual, einoschema.System)
						So(msgs[1].Role, ShouldEqual, einoschema.User)
						So(msgs[2].Role, ShouldEqual, einoschema.Assistant)
						So(len(msgs[2].ToolCalls), ShouldBeGreaterThan, 0)
						So(msgs[3].Role, ShouldEqual, einoschema.Tool)
						So(msgs[3].ToolCallID, ShouldEqual, "call-1")
					},
					output: []*einoschema.Message{
						{
							Role:    einoschema.Assistant,
							Content: "final answer",
							ResponseMeta: &einoschema.ResponseMeta{
								FinishReason: llmchat.FinishReasonStop,
							},
						},
					},
				},
			},
		}

		a := New(AgentConfig[struct{}]{
			MaxRound: 2,
			LLM:      model,
			tools: map[string]einotool.InvokableTool{
				"fake_tool": &fakeInvokableTool{},
			},
		}, struct{}{})

		got, err := a.ReactStream(context.Background(), []*einoschema.Message{
			{Role: einoschema.System, Content: "system"},
			{Role: einoschema.User, Content: "question"},
		})
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Content, ShouldEqual, "final answer")

		accMsgs := a.GetAccumulatedMessages()
		So(len(accMsgs), ShouldEqual, 5)
		So(accMsgs[0].Role, ShouldEqual, einoschema.System)
		So(accMsgs[1].Role, ShouldEqual, einoschema.User)
		So(accMsgs[2].Role, ShouldEqual, einoschema.Assistant)
		So(accMsgs[3].Role, ShouldEqual, einoschema.Tool)
		So(accMsgs[4].Role, ShouldEqual, einoschema.Assistant)
		So(accMsgs[4].Content, ShouldEqual, "final answer")
	})
}

func TestAgentReact_AccumulatesHistoryAcrossToolRounds(t *testing.T) {
	Convey("React 在工具调用轮次后正确累计历史消息", t, func() {
		model := &fakeToolCallingModel{
			rounds: []fakeRound{
				{
					assertInput: func(msgs []*einoschema.Message) {
						So(len(msgs), ShouldEqual, 2)
						So(msgs[0].Role, ShouldEqual, einoschema.System)
						So(msgs[1].Role, ShouldEqual, einoschema.User)
					},
					output: []*einoschema.Message{
						{
							Role: einoschema.Assistant,
							ToolCalls: []einoschema.ToolCall{
								{
									ID:   "call-1",
									Type: "function",
									Function: einoschema.FunctionCall{
										Name:      "fake_tool",
										Arguments: `{"arg":"x"}`,
									},
								},
							},
							ResponseMeta: &einoschema.ResponseMeta{
								FinishReason: llmchat.FinishReasonToolCalls,
							},
						},
					},
				},
				{
					assertInput: func(msgs []*einoschema.Message) {
						So(len(msgs), ShouldEqual, 4)
						So(msgs[0].Role, ShouldEqual, einoschema.System)
						So(msgs[1].Role, ShouldEqual, einoschema.User)
						So(msgs[2].Role, ShouldEqual, einoschema.Assistant)
						So(len(msgs[2].ToolCalls), ShouldBeGreaterThan, 0)
						So(msgs[3].Role, ShouldEqual, einoschema.Tool)
						So(msgs[3].ToolCallID, ShouldEqual, "call-1")
					},
					output: []*einoschema.Message{
						{
							Role:    einoschema.Assistant,
							Content: "final answer",
							ResponseMeta: &einoschema.ResponseMeta{
								FinishReason: llmchat.FinishReasonStop,
							},
						},
					},
				},
			},
		}

		a := New(AgentConfig[struct{}]{
			MaxRound: 2,
			LLM:      model,
			tools: map[string]einotool.InvokableTool{
				"fake_tool": &fakeInvokableTool{},
			},
		}, struct{}{})

		got, err := a.React(context.Background(), []*einoschema.Message{
			{Role: einoschema.System, Content: "system"},
			{Role: einoschema.User, Content: "question"},
		})
		So(err, ShouldBeNil)
		So(got, ShouldNotBeNil)
		So(got.Content, ShouldEqual, "final answer")

		accMsgs := a.GetAccumulatedMessages()
		So(len(accMsgs), ShouldEqual, 5)
		So(accMsgs[0].Role, ShouldEqual, einoschema.System)
		So(accMsgs[1].Role, ShouldEqual, einoschema.User)
		So(accMsgs[2].Role, ShouldEqual, einoschema.Assistant)
		So(accMsgs[3].Role, ShouldEqual, einoschema.Tool)
		So(accMsgs[4].Role, ShouldEqual, einoschema.Assistant)
		So(accMsgs[4].Content, ShouldEqual, "final answer")
	})
}
