package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestSharedRunnerConcurrentSessions(t *testing.T) {
	ctx := context.Background()
	model := newScriptedModel(
		scriptedResponse{msg: assistantMsg("a", nil)},
		scriptedResponse{msg: assistantMsg("b", nil)},
		scriptedResponse{msg: assistantMsg("c", nil)},
		scriptedResponse{msg: assistantMsg("d", nil)},
	)
	runner := mustRunner(t, RunnerConfig{Model: model, Name: "concurrent"})

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			session := NewSession(struct{}{})
			session.Append(schema.UserMessage(fmt.Sprintf("q%d", i)))

			if _, err := runner.Run(ctx, session, RunConfig[struct{}]{}); err != nil {
				t.Errorf("run %d: %v", i, err)
				return
			}
			if len(session.Messages) != 2 {
				t.Errorf("run %d session messages: %d", i, len(session.Messages))
			}
		})
	}
	wg.Wait()
}

// toolThenFinalModel 第一次调用返回工具调用，看到 tool 结果后返回最终答案。
type toolThenFinalModel struct{}

var _ einomodel.ToolCallingChatModel = (*toolThenFinalModel)(nil)

func (m *toolThenFinalModel) Generate(
	ctx context.Context,
	in []*schema.Message,
	opts ...einomodel.Option,
) (*schema.Message, error) {
	for i := len(in) - 1; i >= 0; i-- {
		if in[i].Role == schema.Tool {
			return assistantMsg("done", nil), nil
		}
	}
	return toolCallAssistant("", newToolCall("c1", "echo", "{}")), nil
}

func (m *toolThenFinalModel) Stream(
	ctx context.Context,
	in []*schema.Message,
	opts ...einomodel.Option,
) (*schema.StreamReader[*schema.Message], error) {
	msg, err := m.Generate(ctx, in, opts...)
	if err != nil {
		return nil, err
	}
	reader, writer := schema.Pipe[*schema.Message](0)
	go func() {
		defer writer.Close()
		writer.Send(msg, nil)
	}()
	return reader, nil
}

func (m *toolThenFinalModel) WithTools(
	tools []*schema.ToolInfo,
) (einomodel.ToolCallingChatModel, error) {
	return m, nil
}

func TestSharedRunnerConcurrentToolSessions(t *testing.T) {
	ctx := context.Background()
	runner := mustRunner(t, RunnerConfig{
		Model: &toolThenFinalModel{},
		Tools: []Tool{&fakeTool{name: "echo", run: func(context.Context, string) (string, error) {
			return "ok", nil
		}}},
	})

	var wg sync.WaitGroup
	for i := range 4 {
		wg.Go(func() {
			session := NewSession(struct{}{})
			session.Append(schema.UserMessage(fmt.Sprintf("q%d", i)))

			res, err := runner.Run(ctx, session, RunConfig[struct{}]{})
			if err != nil {
				t.Errorf("run %d: %v", i, err)
				return
			}
			if res.Final.Content != "done" {
				t.Errorf("run %d final: %+v", i, res.Final)
			}
			if len(session.Messages) != 4 {
				t.Errorf("run %d messages: %d", i, len(session.Messages))
			}
		})
	}
	wg.Wait()
}
