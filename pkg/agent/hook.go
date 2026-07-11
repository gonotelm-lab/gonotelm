package agent

import (
	"context"

	einoschema "github.com/cloudwego/eino/schema"
)

const FinalRoundInstruction = "IMPORTANT: 这轮输出是你最后一轮输出，请直接输出最终结果，**不需要再进行工具调用**，按照你已有的信息输出最终结果"

func NewFinalRoundHook[T any](
	ag *Agent[T],
	maxRound int,
) BeforeRoundHook[T] {
	return func(
		_ context.Context,
		round int,
		_ T,
		msgs []*einoschema.Message,
	) ([]*einoschema.Message, error) {
		if round >= maxRound-1 {
			msgs = append(msgs, &einoschema.Message{
				Role:    einoschema.User,
				Content: FinalRoundInstruction,
			})
			ag.StripTools()
		}

		return msgs, nil
	}
}
