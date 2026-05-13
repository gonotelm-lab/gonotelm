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

type AgentConfig[T any] struct {
	MaxRound int

	// 带工具的大模型
	LLM eino.ToolCallingChatModel

	// 每次向模型发起请求时附带的动态参数（如是否开启思考）
	Options []eino.Option

	tools map[string]einotool.InvokableTool

	// 一般可以在此hook中注入系统提示词等操作 如果超过上下文还可以进行上下文压缩等操作
	BeforeChat  AgentBeforeChatHook[T]
	BeforeRound AgentBeforeRoundHook[T]

	MsgAppender func(ctx context.Context, state T, newMsgs []*einoschema.Message)

	OnReasoning    AgentStreamingHook[T]
	OnReasoningEnd AgentStreamingHook[T]
	OnContent      AgentStreamingHook[T]
}

type AgentBeforeChatHook[T any] func(
	ctx context.Context, state T, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

type AgentBeforeRoundHook[T any] func(
	ctx context.Context, round int, state T, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

type AgentStreamingHook[T any] func(
	ctx context.Context,
	round int,
	msg *einoschema.Message,
	state T,
) error

// agent for chat logic
type Agent[T any] struct {
	cfg   AgentConfig[T]
	state T
}

func NewAgent[T any](cfg AgentConfig[T], state T) *Agent[T] {
	if cfg.MaxRound <= 0 {
		cfg.MaxRound = defaultAgentRound
	}

	return &Agent[T]{cfg: cfg, state: state}
}

func (a *Agent[T]) BindTools(tools map[string]einotool.InvokableTool) {
	a.cfg.tools = tools
}

func (a *Agent[T]) Generate(
	ctx context.Context,
	msgs []*einoschema.Message,
) (*einoschema.Message, error) {
	if len(msgs) == 0 {
		return nil, errors.ErrParams.Msg("no messages to chat")
	}

	if a.cfg.BeforeChat != nil {
		newMsgs, err := a.cfg.BeforeChat(ctx, a.state, msgs)
		if err != nil {
			return nil, errors.WithMessage(err, "before chat failed")
		}
		msgs = newMsgs
	}

	for round := range a.cfg.MaxRound {
		if a.cfg.BeforeRound != nil {
			newMsgs, err := a.cfg.BeforeRound(ctx, round, a.state, msgs)
			if err != nil {
				return nil, errors.WithMessagef(err, "before round %d failed", round)
			}
			msgs = newMsgs
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
					newMsgs := make([]*einoschema.Message, 0, 1+len(toolMsgs))
					newMsgs = append(newMsgs, msg)
					newMsgs = append(newMsgs, toolMsgs...)
					a.cfg.MsgAppender(ctx, a.state, newMsgs)
				} else {
					// 认为已经结束
					a.cfg.MsgAppender(ctx, a.state, []*einoschema.Message{msg})
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

// 处理工具调用 并且以message的格式返回工具调用的结果
func (a *Agent[T]) handleToolCalls(
	ctx context.Context,
	toolCalls []einoschema.ToolCall,
) []*einoschema.Message {
	var (
		wg      sync.WaitGroup
		results = make([]*einoschema.Message, len(toolCalls))
	)

	if len(toolCalls) == 0 {
		return nil
	}

	for idx, tc := range toolCalls {
		wg.Go(func() {
			results[idx] = &einoschema.Message{
				Role:       einoschema.Tool,
				ToolCallID: tc.ID,
				ToolName:   tc.Function.Name,
			}

			defer func() {
				if e := recover(); e != nil {
					slog.ErrorContext(ctx, "handle tool call panic", slog.Any("err", e))
					results[idx] = &einoschema.Message{
						Role:    einoschema.Tool,
						Content: fmt.Sprintf("tool call panic: %v", e),
					}
				}
			}()

			if a.cfg.tools != nil {
				if invokable, ok := a.cfg.tools[tc.Function.Name]; !ok {
					results[idx].Content = fmt.Sprintf("tool %s not found", tc.Function.Name)
				} else {
					result, err := invokable.InvokableRun(ctx, tc.Function.Arguments)
					if err != nil {
						results[idx].Content = fmt.Sprintf("tool call failed: %v", err)
					} else {
						results[idx].Content = result
					}
				}
			}
		})
	}

	wg.Wait()

	return results
}
