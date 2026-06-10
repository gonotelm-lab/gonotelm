package agent

import (
	"context"
	"fmt"
	"log/slog"
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

type AgentConfig[State any] struct {
	MaxRound int

	// 带工具的大模型
	LLM eino.ToolCallingChatModel

	// 每次向模型发起请求时附带的动态参数（如是否开启思考）
	Options []eino.Option

	tools map[string]einotool.InvokableTool

	// 一般可以在此hook中注入系统提示词等操作 如果超过上下文还可以进行上下文压缩等操作
	BeforeChat  BeforeChatHook[State]
	BeforeRound BeforeRoundHook[State]

	MsgAppender func(ctx context.Context, state State, newMsgs []*einoschema.Message)

	// 流式输出时生效 非流式输出时不会调用hook
	OnReasoning    StreamingHook[State]
	OnReasoningEnd StreamingHook[State]
	OnContent      StreamingHook[State]

	// 由于工具是并发被调用 所以下面的hook也会被并发调用 需要注意并发安全
	BeforeToolCall BeforeToolCallHook[State]
	AfterToolCall  AfterToolCallHook[State]
}

type BeforeChatHook[T any] func(
	ctx context.Context, state T, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

type BeforeRoundHook[T any] func(
	ctx context.Context, round int, state T, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

type StreamingHook[T any] func(
	ctx context.Context,
	round int,
	msg *einoschema.Message,
	state T,
) error

type BeforeToolCallHook[T any] func(
	ctx context.Context,
	state T,
	tool string,
	arguments string,
)

type AfterToolCallHookResult struct {
	Result string
	Error  error
}

type AfterToolCallHook[T any] func(
	ctx context.Context,
	state T,
	tool string,
	result *AfterToolCallHookResult,
)

// agent for chat logic
type Agent[State any] struct {
	cfg   AgentConfig[State]
	state State

	accMsgs []*einoschema.Message // 累计的历史消息
}

func New[State any](cfg AgentConfig[State], state State) *Agent[State] {
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

func (a *Agent[State]) GetAccumulatedMessages() []*einoschema.Message {
	return a.accMsgs
}

func (a *Agent[State]) setAccumulatedMessages(msgs []*einoschema.Message) {
	if len(msgs) == 0 {
		a.accMsgs = nil
		return
	}
	a.accMsgs = append(a.accMsgs[:0], msgs...)
}

func (a *Agent[State]) appendAccumulatedMessages(msgs ...*einoschema.Message) {
	if len(msgs) == 0 {
		return
	}
	a.accMsgs = append(a.accMsgs, msgs...)
}

// 与模型交互 并返回最终的回答
func (a *Agent[State]) ReactStream(
	ctx context.Context,
	msgs []*einoschema.Message,
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

		// stream handling state
		var (
			finishErr   error
			finished    bool
			finishedMsg *einoschema.Message
		)

		chat.HandleStreamWithCallback(ctx, stream, &chat.Callbacks{
			OnReasoning: func(msg *einoschema.Message) {
				if a.cfg.OnReasoning != nil {
					a.cfg.OnReasoning(ctx, round, msg, a.state)
				}
			},
			OnReasoningEnd: func(msg *einoschema.Message) {
				if a.cfg.OnReasoningEnd != nil {
					a.cfg.OnReasoningEnd(ctx, round, msg, a.state)
				}
			},
			OnContent: func(msg *einoschema.Message) {
				if a.cfg.OnContent != nil {
					a.cfg.OnContent(ctx, round, msg, a.state)
				}
			},
			OnError: func(err error) {
				finishErr = err
			},
			OnEnd: func(msg *einoschema.Message) {
				if msg.ResponseMeta.FinishReason == chat.FinishReasonToolCalls {
					// 需要处理工具调用
					toolMsgs := a.handleToolCalls(ctx, msg.ToolCalls)
					roundMsgs := make([]*einoschema.Message, 0, 1+len(toolMsgs))
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
						a.cfg.MsgAppender(ctx, a.state, []*einoschema.Message{msg})
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
	msgs []*einoschema.Message,
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

		responseMsg, err := a.cfg.LLM.Generate(ctx, msgs, a.cfg.Options...)
		if err != nil {
			return nil, errors.WithMessage(err, "generate chat failed")
		}

		if responseMsg.ResponseMeta.FinishReason == chat.FinishReasonToolCalls {
			toolMsgs := a.handleToolCalls(ctx, responseMsg.ToolCalls)
			roundMsgs := make([]*einoschema.Message, 0, 1+len(toolMsgs))
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
	msgs []*einoschema.Message,
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
	ctx context.Context, round int, msgs []*einoschema.Message,
) ([]*einoschema.Message, error) {
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
) []*einoschema.Message {
	if len(toolCalls) == 0 {
		return nil
	}

	var (
		wg      sync.WaitGroup
		results = make([]*einoschema.Message, len(toolCalls))
	)

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
					slog.ErrorContext(ctx, "handle tool call panic", slog.Any("err", e))
					results[idx] = &einoschema.Message{
						Role:    einoschema.Tool,
						Content: fmt.Sprintf("tool call panic: %v", e),
					}
				}
			}()

			if invokable, ok := a.cfg.tools[tc.Function.Name]; !ok {
				results[idx].Content = fmt.Sprintf("tool %s not found", tc.Function.Name)
			} else {
				if a.cfg.BeforeToolCall != nil {
					a.cfg.BeforeToolCall(ctx, a.state, tc.Function.Name, tc.Function.Arguments)
				}
				result, err := invokable.InvokableRun(ctx, tc.Function.Arguments)
				if err != nil {
					results[idx].Content = fmt.Sprintf("tool call failed: %v", err)
				} else {
					results[idx].Content = result
				}

				if a.cfg.AfterToolCall != nil {
					a.cfg.AfterToolCall(ctx, a.state, tc.Function.Name, &AfterToolCallHookResult{
						Result: result,
						Error:  err,
					})
				}
			}
		})
	}

	wg.Wait()

	return results
}
