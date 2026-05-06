package logic

import (
	"context"
	stderr "errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	eino "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	defaultChatAgentRound = 30
)

type ChatAgentConfig struct {
	MaxRound int

	// 带工具的大模型
	LLM eino.ToolCallingChatModel

	// 一般可以在此hook中注入系统提示词等操作 如果超过上下文还可以进行上下文压缩等操作
	BeforeChat ChatAgentBeforeChatHook

	Tools map[string]einotool.InvokableTool
}

type ChatAgentBeforeChatHook func(
	ctx context.Context, chatId uuid.UUID, msgs []*einoschema.Message,
) ([]*einoschema.Message, error)

// ChatAgent for chat logic
type ChatAgent struct {
	c       ChatAgentConfig
	chatBiz *bizchat.Biz
}

func NewChatAgent(c ChatAgentConfig, llm eino.ToolCallingChatModel) *ChatAgent {
	if c.MaxRound <= 0 {
		c.MaxRound = defaultChatAgentRound
	}

	return &ChatAgent{
		c: c,
	}
}

func (a *ChatAgent) Do(ctx context.Context, chatId uuid.UUID, msgs []*einoschema.Message) error {
	userId := pkgcontext.GetUserId(ctx)
	if a.c.BeforeChat != nil {
		newMsgs, err := a.c.BeforeChat(ctx, chatId, msgs)
		if err != nil {
			return errors.WithMessage(err, "before chat hook failed")
		}

		msgs = newMsgs
	}

	if len(msgs) == 0 {
		return errors.ErrParams.Msg("no messages to chat")
	}

	for round := range a.c.MaxRound {
		stream, err := a.c.LLM.Stream(ctx, msgs)
		if err != nil {
			return errors.WithMessage(err, "stream chat failed")
		}
		defer stream.Close()

		streamProcess := chat.HandleStream(ctx, stream)
		msgs, err = a.doLoop(ctx, round, userId, chatId, streamProcess, msgs)
		if err != nil {
			if stderr.Is(err, io.EOF) {
				return nil
			}

			return errors.WithMessage(err, "chat loop failed")
		}
	}

	return errors.ErrParams.Msgf("chat round exceeded max rounds=%d", a.c.MaxRound)
}

func (a *ChatAgent) doLoop(
	ctx context.Context,
	round int,
	userId string,
	chatId uuid.UUID,
	streamProcess *chat.HandleStreamResult,
	msgs []*einoschema.Message,
) ([]*einoschema.Message, error) {

	return nil, nil
}

func (a *ChatAgent) handleToolCalls(
	ctx context.Context,
	chatId uuid.UUID,
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

			if invokable, ok := a.c.Tools[tc.Function.Name]; !ok {
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
