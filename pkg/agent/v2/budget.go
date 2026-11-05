package agent

import (
	"sync/atomic"

	"github.com/cloudwego/eino/schema"
)

// RunBudget 是一次运行或一组子运行共享的资源上限，并发安全。
//
// 它是进程内的运行护栏，只统计 token 与工具调用次数；不持久化、不识别用户，
// 也不是用户级配额。用户级配额应在 application 层预检查，并用 Result.Usage 结算。
// 0 表示不限制对应维度。
type RunBudget struct {
	maxTokens    int64
	maxToolCalls int64
	tokens       atomic.Int64
	toolCalls    atomic.Int64
}

func NewRunBudget(maxTokens, maxToolCalls int) *RunBudget {
	return &RunBudget{
		maxTokens:    int64(maxTokens),
		maxToolCalls: int64(maxToolCalls),
	}
}

// AddUsage 累加用量并在超限时返回 ErrBudgetExceeded。
func (b *RunBudget) AddUsage(usage schema.TokenUsage) error {
	if b == nil {
		return nil
	}

	n := b.tokens.Add(int64(tokenTotal(usage)))
	if b.maxTokens > 0 && n > b.maxTokens {
		return ErrBudgetExceeded
	}
	return nil
}

// AddToolCall 累加一次工具调用并在超限时返回 ErrBudgetExceeded。
func (b *RunBudget) AddToolCall() error {
	return b.AddToolCallFor(1)
}

// AddToolCallFor 累加 n 次工具调用并在超限时返回 ErrBudgetExceeded。
func (b *RunBudget) AddToolCallFor(n int) error {
	if b == nil || n <= 0 {
		return nil
	}

	got := b.toolCalls.Add(int64(n))
	if b.maxToolCalls > 0 && got > b.maxToolCalls {
		return ErrBudgetExceeded
	}
	return nil
}
