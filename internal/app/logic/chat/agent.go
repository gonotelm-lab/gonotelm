package chat

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	eino "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	defaultAgentRound = 20
)

type agentConfig[T any] struct {
	maxRound int

	// 带工具的大模型
	llm eino.ToolCallingChatModel

	// 一般可以在此hook中注入系统提示词等操作 如果超过上下文还可以进行上下文压缩等操作
	beforeChat  agentBeforeChatHook[T]
	beforeRound agentBeforeRoundHook[T]

	tools map[string]einotool.InvokableTool

	msgAppender func(ctx context.Context, chatId uuid.UUID, newMsgs []*einoschema.Message)

	onReasoning    agentStreamingHook[T]
	onReasoningEnd agentStreamingHook[T]
	onContent      agentStreamingHook[T]
	// onToolsCalled  func(ctx context.Context, chatId uuid.UUID, toolCalls []einoschema.ToolCall)
}

type agentBeforeChatHook[T any] func(
	ctx context.Context,  state T, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

type agentBeforeRoundHook[T any] func(
	ctx context.Context, round int, state T, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

type agentStreamingHook[T any] func(
	ctx context.Context,
	round int,
	msg *einoschema.Message,
	state T,
) error

// agent for chat logic
type agent[T any] struct {
	cfg   agentConfig[T]
	state T
}

func newAgent[T any](cfg agentConfig[T], state T) *agent[T] {
	if cfg.maxRound <= 0 {
		cfg.maxRound = defaultAgentRound
	}

	return &agent[T]{cfg: cfg, state: state}
}

func (a *agent[T]) produceAnswer(
	ctx context.Context,
	chatId uuid.UUID,
	msgs []*einoschema.Message,
) (*einoschema.Message, error) {
	if len(msgs) == 0 {
		return nil, errors.ErrParams.Msg("no messages to chat")
	}

	if a.cfg.beforeChat != nil {
		newMsgs, err := a.cfg.beforeChat(ctx, a.state, msgs)
		if err != nil {
			return nil, errors.WithMessage(err, "before chat failed")
		}
		msgs = newMsgs
	}

	for round := range a.cfg.maxRound {
		if a.cfg.beforeRound != nil {
			newMsgs, err := a.cfg.beforeRound(ctx, round, a.state, msgs)
			if err != nil {
				return nil, errors.WithMessagef(err, "before round %d failed", round)
			}
			msgs = newMsgs
		}

		stream, err := a.cfg.llm.Stream(ctx, msgs)
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
				if a.cfg.onReasoning != nil {
					a.cfg.onReasoning(ctx, round, msg, a.state)
				}
			},
			OnReasoningEnd: func(msg *einoschema.Message) {
				if a.cfg.onReasoningEnd != nil {
					a.cfg.onReasoningEnd(ctx, round, msg, a.state)
				}
			},
			OnContent: func(msg *einoschema.Message) {
				if a.cfg.onContent != nil {
					a.cfg.onContent(ctx, round, msg, a.state)
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
					a.cfg.msgAppender(ctx, chatId, newMsgs)
				} else {
					// 认为已经结束
					debugStr := fmt.Sprintf("chat agent for chat %s ended, reason=%s", chatId.String(),
						msg.ResponseMeta.FinishReason)
					slog.DebugContext(ctx, debugStr)

					a.cfg.msgAppender(ctx, chatId, []*einoschema.Message{msg})
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

	return nil, errors.ErrParams.Msgf("chat round exceeded max rounds=%d", a.cfg.maxRound)
}

// 处理工具调用 并且以message的格式返回工具调用的结果
func (a *agent[T]) handleToolCalls(
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
		})
	}

	wg.Wait()

	return results
}
