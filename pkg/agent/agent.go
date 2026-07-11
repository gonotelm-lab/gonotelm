package agent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	chatstream "github.com/gonotelm-lab/gonotelm/pkg/eino-ext/stream"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/llm"

	eino "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	defaultAgentRound = 20
)

type EinoMessage = einoschema.Message

type Config[State any] struct {
	MaxRound int

	// 基础模型对象 不带工具 工具动态绑定
	BaseLLM eino.ToolCallingChatModel

	// 每次向模型发起请求时附带的动态参数（如是否开启思考）
	Options []eino.Option

	Verbose bool
}

type (
	ToolCall struct {
		Id        string
		Name      string
		Arguments string
	}

	ToolCallResult struct {
		Id        string
		Name      string
		Arguments string
		Result    string
		Error     error
	}
)

type (
	OnDeltaHook[T any]        func(ctx context.Context, round int, state T, delta string) error
	OnStartHook[T any]        func(ctx context.Context, round int, state T) error
	OnEndHook[T any]          func(ctx context.Context, round int, state T) error
	BeforeChatHook[T any]     func(ctx context.Context, state T, msgs []*EinoMessage) ([]*EinoMessage, error)
	BeforeRoundHook[T any]    func(ctx context.Context, round int, state T, msgs []*EinoMessage) ([]*EinoMessage, error)
	BeforeToolCallHook[T any] func(ctx context.Context, state T, toolCalls []ToolCall)
	MsgAppender[T any]        func(ctx context.Context, state T, newMsgs []*EinoMessage)
	AfterToolCallHook[T any]  func(ctx context.Context, state T, results []*ToolCallResult)
)

// agent for chat logic
type Agent[State any] struct {
	cfg   Config[State]
	state State

	tooledLLM eino.ToolCallingChatModel
	tools     map[string]einotool.InvokableTool

	accMsgs []*EinoMessage         // 累计的历史消息
	usage   *einoschema.TokenUsage // 累计token使用

	// 全部工具被调用前回调
	beforeToolCall BeforeToolCallHook[State]
	// 全部工具被调用后回调
	afterToolCall AfterToolCallHook[State]

	msgAppender MsgAppender[State]

	// 流式输出时生效 非流式输出时不会调用hook
	onReasoningStart OnStartHook[State]
	onReasoningDelta OnDeltaHook[State]
	onReasoningEnd   OnEndHook[State]
	onContentStart   OnStartHook[State]
	onContentDelta   OnDeltaHook[State]
	onContentEnd     OnEndHook[State]

	// 一般可以在此hook中注入系统提示词等操作 如果超过上下文还可以进行上下文压缩等操作
	beforeChat  BeforeChatHook[State]
	beforeRound BeforeRoundHook[State]
}

func New[State any](cfg Config[State], state State) *Agent[State] {
	if cfg.MaxRound <= 0 {
		cfg.MaxRound = defaultAgentRound
	}

	initUsage := &einoschema.TokenUsage{
		PromptTokens:            0,
		PromptTokenDetails:      einoschema.PromptTokenDetails{},
		CompletionTokens:        0,
		TotalTokens:             0,
		CompletionTokensDetails: einoschema.CompletionTokensDetails{},
	}

	return &Agent[State]{
		cfg:       cfg,
		state:     state,
		tooledLLM: cfg.BaseLLM,
		usage:     initUsage,
	}
}

func (a *Agent[State]) OnReasoningStart(hook OnStartHook[State]) {
	a.onReasoningStart = hook
}

func (a *Agent[State]) OnReasoningDelta(hook OnDeltaHook[State]) {
	a.onReasoningDelta = hook
}

func (a *Agent[State]) OnReasoningEnd(hook OnEndHook[State]) {
	a.onReasoningEnd = hook
}

func (a *Agent[State]) OnContentStart(hook OnStartHook[State]) {
	a.onContentStart = hook
}

func (a *Agent[State]) OnContentDelta(hook OnDeltaHook[State]) {
	a.onContentDelta = hook
}

func (a *Agent[State]) OnContentEnd(hook OnEndHook[State]) {
	a.onContentEnd = hook
}

func (a *Agent[State]) OnBeforeChat(hook BeforeChatHook[State]) {
	a.beforeChat = hook
}

func (a *Agent[State]) OnBeforeRound(hook BeforeRoundHook[State]) {
	a.beforeRound = hook
}

func (a *Agent[State]) OnBeforeToolCall(hook BeforeToolCallHook[State]) {
	a.beforeToolCall = hook
}

func (a *Agent[State]) OnAfterToolCall(hook AfterToolCallHook[State]) {
	a.afterToolCall = hook
}

func (a *Agent[State]) OnMsgAppender(hook MsgAppender[State]) {
	a.msgAppender = hook
}

func (a *Agent[State]) BindTools(tools map[string]einotool.InvokableTool) error {
	if len(tools) == 0 {
		return nil
	}

	a.tools = tools
	toolInfos := make([]*einoschema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		toolInfo, err := tool.Info(context.Background())
		if err != nil {
			continue
		}
		toolInfos = append(toolInfos, toolInfo)
	}

	tooledLLM, err := a.cfg.BaseLLM.WithTools(toolInfos)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "bind tools failed: %v", err)
	}
	a.tooledLLM = tooledLLM

	return nil
}

// 清空所有已绑定的工具
// 可以在React循环中动态删减工具
func (a *Agent[State]) StripTools() {
	a.tooledLLM = a.cfg.BaseLLM
	clear(a.tools)
}

func (a *Agent[State]) AccumulatedMessages() []*EinoMessage {
	return a.accMsgs
}

func (a *Agent[State]) TokenUsage() einoschema.TokenUsage {
	return *a.usage
}

func (a *Agent[State]) BaseLLM() eino.ToolCallingChatModel {
	return a.cfg.BaseLLM
}

func (a *Agent[State]) setAccumulatedMessages(msgs []*EinoMessage) {
	if len(msgs) == 0 {
		a.accMsgs = nil
		return
	}
	a.accMsgs = append(a.accMsgs[:0], msgs...)
}

func (a *Agent[State]) appendAccumulatedMessages(msgs ...*EinoMessage) {
	if len(msgs) == 0 {
		return
	}
	a.accMsgs = append(a.accMsgs, msgs...)
}

// 与模型交互 并返回最终的回答
func (a *Agent[State]) ReactStream(
	ctx context.Context,
	msgs []*EinoMessage,
) (*einoschema.Message, error) {
	if len(msgs) == 0 {
		return nil, errors.ErrParams.Msg("no messages to chat")
	}
	a.setAccumulatedMessages(msgs)

	msgs, err := a.handleBeforeChat(ctx, msgs)
	if err != nil {
		return nil, errors.WithMessage(err, "handle before chat failed")
	}

	for round := range a.cfg.MaxRound {
		msgs, err = a.handleBeforeRound(ctx, round, msgs)
		if err != nil {
			return nil, errors.WithMessagef(err, "before round %d failed", round)
		}

		stream, err := a.tooledLLM.Stream(ctx, msgs, a.cfg.Options...)
		if err != nil {
			return nil, errors.WithMessage(err, "stream chat failed")
		}
		defer stream.Close()

		var (
			finishErr   error
			finished    bool
			finishedMsg *EinoMessage
		)

		// 处理流式消息
		chatstream.HandleStreamWithCallback(ctx, stream, &chatstream.Callbacks{
			OnReasoningStart: func() {
				if a.onReasoningStart != nil {
					a.onReasoningStart(ctx, round, a.state)
				}
			},
			OnReasoningDelta: func(delta string) {
				if a.onReasoningDelta != nil {
					a.onReasoningDelta(ctx, round, a.state, delta)
				}
			},
			OnReasoningEnd: func() {
				if a.onReasoningEnd != nil {
					a.onReasoningEnd(ctx, round, a.state)
				}
			},
			OnContentStart: func() {
				if a.onContentStart != nil {
					a.onContentStart(ctx, round, a.state)
				}
			},
			OnContentDelta: func(delta string) {
				if a.onContentDelta != nil {
					a.onContentDelta(ctx, round, a.state, delta)
				}
			},
			OnContentEnd: func() {
				if a.onContentEnd != nil {
					a.onContentEnd(ctx, round, a.state)
				}
			},
			OnError: func(err error) {
				finishErr = err
			},
			OnDone: func(msg *EinoMessage) {
				a.updateTokenUsage(msg.ResponseMeta.Usage)
				if msg.ResponseMeta.FinishReason == llm.FinishReasonToolCalls {
					// 需要处理工具调用
					toolMsgs := a.handleToolCalls(ctx, msg.ToolCalls)
					roundMsgs := make([]*EinoMessage, 0, 1+len(toolMsgs))
					roundMsgs = append(roundMsgs, msg)
					roundMsgs = append(roundMsgs, toolMsgs...)
					msgs = append(msgs, roundMsgs...)
					a.appendAccumulatedMessages(roundMsgs...)
					if a.msgAppender != nil {
						a.msgAppender(ctx, a.state, roundMsgs)
					}

				} else {
					// 认为已经结束
					msgs = append(msgs, msg)
					a.appendAccumulatedMessages(msg)
					if a.msgAppender != nil {
						a.msgAppender(ctx, a.state, []*EinoMessage{msg})
					}
					finished = true
					finishedMsg = msg
				}
			},
		})

		if finishErr != nil {
			return nil, finishErr
		}

		if finished {
			// 已经得到结果了
			return finishedMsg, nil
		}
	}

	return nil, errors.ErrParams.Msgf("chat round exceeded max rounds=%d", a.cfg.MaxRound)
}

// 非流式输出
func (a *Agent[State]) React(
	ctx context.Context,
	msgs []*EinoMessage,
) (*EinoMessage, error) {
	if len(msgs) == 0 {
		return nil, errors.ErrParams.Msg("no messages to chat")
	}
	a.setAccumulatedMessages(msgs)

	msgs, err := a.handleBeforeChat(ctx, msgs)
	if err != nil {
		return nil, errors.WithMessage(err, "handle before chat failed")
	}

	for round := range a.cfg.MaxRound {
		msgs, err = a.handleBeforeRound(ctx, round, msgs)
		if err != nil {
			return nil, errors.WithMessagef(err, "before round %d failed", round)
		}

		responseMsg, err := a.tooledLLM.Generate(ctx, msgs, a.cfg.Options...)
		if err != nil {
			return nil, errors.WithMessage(err, "generate chat failed")
		}

		a.updateTokenUsage(responseMsg.ResponseMeta.Usage)

		if responseMsg.ResponseMeta.FinishReason == llm.FinishReasonToolCalls {
			toolMsgs := a.handleToolCalls(ctx, responseMsg.ToolCalls)
			roundMsgs := make([]*EinoMessage, 0, 1+len(toolMsgs))
			roundMsgs = append(roundMsgs, responseMsg)
			roundMsgs = append(roundMsgs, toolMsgs...)
			msgs = append(msgs, roundMsgs...)
			a.appendAccumulatedMessages(roundMsgs...)
			if a.msgAppender != nil {
				a.msgAppender(ctx, a.state, roundMsgs)
			}
		} else {
			// 没有工具调用任务 认为已经结束
			a.appendAccumulatedMessages(responseMsg)
			if a.msgAppender != nil {
				a.msgAppender(ctx, a.state, []*EinoMessage{responseMsg})
			}
			return responseMsg, nil
		}
	}

	return nil, errors.ErrParams.Msgf("chat round exceeded max rounds=%d", a.cfg.MaxRound)
}

func (a *Agent[State]) updateTokenUsage(usage *einoschema.TokenUsage) {
	a.usage.PromptTokens += usage.PromptTokens
	a.usage.PromptTokenDetails.CachedTokens += usage.PromptTokenDetails.CachedTokens
	a.usage.CompletionTokens += usage.CompletionTokens
	a.usage.TotalTokens += usage.TotalTokens
	a.usage.CompletionTokensDetails.ReasoningTokens += usage.CompletionTokensDetails.ReasoningTokens
}

func (a *Agent[State]) handleBeforeChat(
	ctx context.Context,
	msgs []*EinoMessage,
) ([]*EinoMessage, error) {
	if a.beforeChat != nil {
		newMsgs, err := a.beforeChat(ctx, a.state, msgs)
		if err != nil {
			return nil, errors.WithMessage(err, "before chat failed")
		}

		a.setAccumulatedMessages(newMsgs)
		return newMsgs, nil
	}

	return msgs, nil
}

func (a *Agent[State]) handleBeforeRound(
	ctx context.Context, round int, msgs []*EinoMessage,
) ([]*EinoMessage, error) {
	if a.beforeRound != nil {
		newMsgs, err := a.beforeRound(ctx, round, a.state, msgs)
		if err != nil {
			return nil, errors.WithMessage(err, "before round failed")
		}

		a.setAccumulatedMessages(newMsgs)
		return newMsgs, nil
	}

	return msgs, nil
}

// 处理工具调用 并且以message的格式返回工具调用的结果
func (a *Agent[State]) handleToolCalls(
	ctx context.Context,
	toolCalls []einoschema.ToolCall,
) []*EinoMessage {
	if len(toolCalls) == 0 {
		return nil
	}

	var (
		wg                sync.WaitGroup
		results           = make([]*EinoMessage, len(toolCalls))
		resultForCallback = make([]*ToolCallResult, len(toolCalls))
	)
	for i := range resultForCallback {
		resultForCallback[i] = &ToolCallResult{}
	}

	tcs := make([]ToolCall, 0, len(toolCalls))
	for _, tc := range toolCalls {
		tcs = append(tcs, ToolCall{
			Id:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}
	if a.beforeToolCall != nil {
		a.beforeToolCall(ctx, a.state, tcs)
	}

	for idx, toolCall := range toolCalls {
		wg.Go(func() {
			results[idx] = &einoschema.Message{
				Role:       einoschema.Tool,
				ToolCallID: toolCall.ID,
				ToolName:   toolCall.Function.Name,
			}

			if a.cfg.Verbose {
				slog.DebugContext(ctx, "handling tool call",
					slog.String("tool_name", toolCall.Function.Name),
					slog.String("tool_call_id", toolCall.ID),
					slog.String("tool_call_arguments", string(toolCall.Function.Arguments)),
				)
			}

			defer func() {
				if e := recover(); e != nil {
					panicErr := fmt.Errorf("tool call panic: %v", e)
					resultForCallback[idx].Error = panicErr
					slog.ErrorContext(ctx,
						"handle tool call panic",
						slog.Any("err", e),
						slog.String("tool_name", toolCall.Function.Name),
						slog.String("tool_call_id", toolCall.ID),
						slog.String("stack", string(debug.Stack())),
					)
					results[idx].Content = panicErr.Error()
				}
			}()

			invokable, ok := a.tools[toolCall.Function.Name]
			resultForCallback[idx].Id = toolCall.ID
			resultForCallback[idx].Name = toolCall.Function.Name
			resultForCallback[idx].Arguments = toolCall.Function.Arguments

			if !ok {
				err := fmt.Errorf("tool %s not found", toolCall.Function.Name)
				results[idx].Content = err.Error()
				resultForCallback[idx].Error = err
				return
			}

			result, err := invokable.InvokableRun(ctx, toolCall.Function.Arguments)
			if err != nil {
				results[idx].Content = fmt.Sprintf("tool call failed: %v", err)
				resultForCallback[idx].Error = err
				return
			}

			results[idx].Content = result
			resultForCallback[idx].Result = result

			if a.cfg.Verbose {
				slog.DebugContext(ctx, "tool call finished",
					slog.String("tool_name", toolCall.Function.Name),
					slog.String("tool_call_id", toolCall.ID),
					slog.String("tool_call_arguments", string(toolCall.Function.Arguments)),
					slog.String("tool_call_result", result),
				)
			}
		})
	}

	wg.Wait()

	// after took
	if a.afterToolCall != nil {
		a.afterToolCall(ctx, a.state, resultForCallback)
	}

	return results
}
