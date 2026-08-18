package agent

import (
	"context"

	einoschema "github.com/cloudwego/eino/schema"
)

const FinalRoundInstruction = "IMPORTANT: This is your final round. Output the final result directly based on what you already have. **Do not make any more tool calls.**"

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
