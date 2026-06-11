package agent

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

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

	// 带工具的大模型
	LLM eino.ToolCallingChatModel

	// 每次向模型发起请求时附带的动态参数（如是否开启思考）
	Options []eino.Option

	tools map[string]einotool.InvokableTool

	// 一般可以在此hook中注入系统提示词等操作 如果超过上下文还可以进行上下文压缩等操作
	BeforeChat  BeforeChatHook[State]
	BeforeRound BeforeRoundHook[State]

	MsgAppender MsgAppender[State]

	// 全部工具被调用前回调
	BeforeToolCall BeforeToolCallHook[State]
	// 全部工具被调用后回调
	AfterToolCall ToolCallHook[State]

	// 流式输出时生效 非流式输出时不会调用hook
	OnReasoningStart OnStartHook[State]
	OnReasoningDelta OnDeltaHook[State]
	OnReasoningEnd   OnEndHook[State]
	OnContentStart   OnStartHook[State]
	OnContentDelta   OnDeltaHook[State]
	OnContentEnd     OnEndHook[State]
}

type OnDeltaHook[T any] func(ctx context.Context, round int, state T, delta string) error

type OnStartHook[T any] func(ctx context.Context, round int, state T) error

type OnEndHook[T any] func(ctx context.Context, round int, state T) error

type BeforeChatHook[T any] func(ctx context.Context, state T, msgs []*EinoMessage) ([]*EinoMessage, error)

type BeforeRoundHook[T any] func(ctx context.Context, round int, state T, msgs []*EinoMessage) ([]*EinoMessage, error)

type BeforeToolCallHook[T any] func(ctx context.Context, state T, toolCalls []einoschema.ToolCall)

type MsgAppender[T any] func(ctx context.Context, state T, newMsgs []*EinoMessage)

type ToolCallHookResult struct {
	Result string
	Error  error
}

type ToolCallHook[T any] func(
	ctx context.Context,
	state T,
	results []*ToolCallHookResult,
)

// agent for chat logic
type Agent[State any] struct {
	cfg   Config[State]
	state State

	accMsgs []*EinoMessage // 累计的历史消息
}

func New[State any](cfg Config[State], state State) *Agent[State] {
	if cfg.MaxRound <= 0 {
		cfg.MaxRound = defaultAgentRound
	}

	return &Agent[State]{cfg: cfg, state: state}
}

func (a *Agent[State]) BindTools(tools map[string]einotool.InvokableTool) error {
	a.cfg.tools = tools
	toolInfos := make([]*einoschema.ToolInfo, 0, len(tools))
	for _, tool := range tools {
		toolInfo, err := tool.Info(context.Background())
		if err != nil {
			continue
		}
		toolInfos = append(toolInfos, toolInfo)
	}

	toolLLM, err := a.cfg.LLM.WithTools(toolInfos)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "bind tools failed: %v", err)
	}
	a.cfg.LLM = toolLLM

	return nil
}

func (a *Agent[State]) GetAccumulatedMessages() []*EinoMessage {
	return a.accMsgs
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

		stream, err := a.cfg.LLM.Stream(ctx, msgs, a.cfg.Options...)
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
		chat.HandleStreamWithCallback(ctx, stream, &chat.Callbacks{
			OnReasoningStart: func() {
				if a.cfg.OnReasoningStart != nil {
					a.cfg.OnReasoningStart(ctx, round, a.state)
				}
			},
			OnReasoningDelta: func(delta string) {
				if a.cfg.OnReasoningDelta != nil {
					a.cfg.OnReasoningDelta(ctx, round, a.state, delta)
				}
			},
			OnReasoningEnd: func() {
				if a.cfg.OnReasoningEnd != nil {
					a.cfg.OnReasoningEnd(ctx, round, a.state)
				}
			},
			OnContentStart: func() {
				if a.cfg.OnContentStart != nil {
					a.cfg.OnContentStart(ctx, round, a.state)
				}
			},
			OnContentDelta: func(delta string) {
				if a.cfg.OnContentDelta != nil {
					a.cfg.OnContentDelta(ctx, round, a.state, delta)
				}
			},
			OnContentEnd: func() {
				if a.cfg.OnContentEnd != nil {
					a.cfg.OnContentEnd(ctx, round, a.state)
				}
			},
			OnError: func(err error) {
				finishErr = err
			},
			OnDone: func(msg *EinoMessage) {
				if msg.ResponseMeta.FinishReason == chat.FinishReasonToolCalls {
					// 需要处理工具调用
					toolMsgs := a.handleToolCalls(ctx, msg.ToolCalls)
					roundMsgs := make([]*EinoMessage, 0, 1+len(toolMsgs))
					roundMsgs = append(roundMsgs, msg)
					roundMsgs = append(roundMsgs, toolMsgs...)
					msgs = append(msgs, roundMsgs...)
					a.appendAccumulatedMessages(roundMsgs...)
					if a.cfg.MsgAppender != nil {
						a.cfg.MsgAppender(ctx, a.state, roundMsgs)
					}
				} else {
					// 认为已经结束
					msgs = append(msgs, msg)
					a.appendAccumulatedMessages(msg)
					if a.cfg.MsgAppender != nil {
						a.cfg.MsgAppender(ctx, a.state, []*EinoMessage{msg})
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

		responseMsg, err := a.cfg.LLM.Generate(ctx, msgs, a.cfg.Options...)
		if err != nil {
			return nil, errors.WithMessage(err, "generate chat failed")
		}

		if responseMsg.ResponseMeta.FinishReason == chat.FinishReasonToolCalls {
			toolMsgs := a.handleToolCalls(ctx, responseMsg.ToolCalls)
			roundMsgs := make([]*EinoMessage, 0, 1+len(toolMsgs))
			roundMsgs = append(roundMsgs, responseMsg)
			roundMsgs = append(roundMsgs, toolMsgs...)
			msgs = append(msgs, roundMsgs...)
			a.appendAccumulatedMessages(roundMsgs...)
			if a.cfg.MsgAppender != nil {
				a.cfg.MsgAppender(ctx, a.state, roundMsgs)
			}
		} else {
			// 没有工具调用任务 认为已经结束
			a.appendAccumulatedMessages(responseMsg)
			if a.cfg.MsgAppender != nil {
				a.cfg.MsgAppender(ctx, a.state, []*einoschema.Message{responseMsg})
			}
			return responseMsg, nil
		}
	}

	return nil, errors.ErrParams.Msgf("chat round exceeded max rounds=%d", a.cfg.MaxRound)
}

func (a *Agent[State]) handleBeforeChat(
	ctx context.Context,
	msgs []*EinoMessage,
) ([]*einoschema.Message, error) {
	if a.cfg.BeforeChat != nil {
		newMsgs, err := a.cfg.BeforeChat(ctx, a.state, msgs)
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
	if a.cfg.BeforeRound != nil {
		newMsgs, err := a.cfg.BeforeRound(ctx, round, a.state, msgs)
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
		resultForCallback = make([]*ToolCallHookResult, len(toolCalls))
	)
	for i := range resultForCallback {
		resultForCallback[i] = &ToolCallHookResult{}
	}

	if a.cfg.BeforeToolCall != nil {
		a.cfg.BeforeToolCall(ctx, a.state, toolCalls)
	}

	for idx, tc := range toolCalls {
		wg.Go(func() {
			results[idx] = &einoschema.Message{
				Role:       einoschema.Tool,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
			}

			slog.DebugContext(ctx, "handling tool call",
				slog.String("tool_name", tc.Function.Name),
				slog.String("tool_call_id", tc.ID),
				slog.String("tool_call_arguments", string(tc.Function.Arguments)),
			)

			defer func() {
				if e := recover(); e != nil {
					panicErr := fmt.Errorf("tool call panic: %v", e)
					resultForCallback[idx].Error = panicErr
					slog.ErrorContext(ctx,
						"handle tool call panic",
						slog.Any("err", e),
						slog.String("tool_name", tc.Function.Name),
						slog.String("tool_call_id", tc.ID),
						slog.String("stack", string(debug.Stack())),
					)
					results[idx].Content = panicErr.Error()
				}
			}()

			invokable, ok := a.cfg.tools[tc.Function.Name]
			if !ok {
				err := fmt.Errorf("tool %s not found", tc.Function.Name)
				results[idx].Content = err.Error()
				resultForCallback[idx].Error = err
				return
			}

			result, err := invokable.InvokableRun(ctx, tc.Function.Arguments)
			if err != nil {
				results[idx].Content = fmt.Sprintf("tool call failed: %v", err)
				resultForCallback[idx].Error = err
				return
			}

			results[idx].Content = result
			resultForCallback[idx].Result = result

			// slog.DebugContext(ctx, "tool call result",
			// 	slog.String("tool_name", tc.Function.Name),
			// 	slog.String("tool_call_id", tc.ID),
			// 	slog.String("tool_call_arguments", string(tc.Function.Arguments)),
			// 	slog.String("tool_call_result", result),
			// )
		})
	}

	wg.Wait()

	// after took
	if a.cfg.AfterToolCall != nil {
		a.cfg.AfterToolCall(ctx, a.state, resultForCallback)
	}

	return results
}
