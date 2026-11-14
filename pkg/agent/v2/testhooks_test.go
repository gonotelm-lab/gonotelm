package agent

import (
	"context"
)

// testHooks 仅用于测试：嵌入 NoopHooks，按需覆写关心的方法。
type testHooks struct {
	NoopHooks[struct{}]

	ReasoningStart func(ctx context.Context, oc *OutputContext) error
	ReasoningDelta func(ctx context.Context, oc *OutputContext) error
	ReasoningEnd   func(ctx context.Context, oc *OutputContext) error

	ContentStart func(ctx context.Context, oc *OutputContext) error
	ContentDelta func(ctx context.Context, oc *OutputContext) error
	ContentEnd   func(ctx context.Context, oc *OutputContext) error

	ToolCallStart func(ctx context.Context, tc *ToolCallContext[struct{}]) error
	ToolCallEnd   func(ctx context.Context, tc *ToolCallContext[struct{}]) error

	RunStart      func(ctx context.Context, rc *RunContext[struct{}]) error
	RoundStart    func(ctx context.Context, rc *RoundContext[struct{}]) error
	RoundComplete func(ctx context.Context, rc *RoundContext[struct{}]) error
	RoundEnd      func(ctx context.Context, rc *RoundContext[struct{}]) error
	RunEnd        func(ctx context.Context, rc *RunContext[struct{}])
}

var _ Hooks[struct{}] = testHooks{}

func (h testHooks) OnReasoningStart(ctx context.Context, oc *OutputContext) error {
	if h.ReasoningStart == nil {
		return nil
	}
	return h.ReasoningStart(ctx, oc)
}

func (h testHooks) OnReasoningDelta(ctx context.Context, oc *OutputContext) error {
	if h.ReasoningDelta == nil {
		return nil
	}
	return h.ReasoningDelta(ctx, oc)
}

func (h testHooks) OnReasoningEnd(ctx context.Context, oc *OutputContext) error {
	if h.ReasoningEnd == nil {
		return nil
	}
	return h.ReasoningEnd(ctx, oc)
}

func (h testHooks) OnContentStart(ctx context.Context, oc *OutputContext) error {
	if h.ContentStart == nil {
		return nil
	}
	return h.ContentStart(ctx, oc)
}

func (h testHooks) OnContentDelta(ctx context.Context, oc *OutputContext) error {
	if h.ContentDelta == nil {
		return nil
	}
	return h.ContentDelta(ctx, oc)
}

func (h testHooks) OnContentEnd(ctx context.Context, oc *OutputContext) error {
	if h.ContentEnd == nil {
		return nil
	}
	return h.ContentEnd(ctx, oc)
}

func (h testHooks) OnToolCallStart(ctx context.Context, tc *ToolCallContext[struct{}]) error {
	if h.ToolCallStart == nil {
		return nil
	}
	return h.ToolCallStart(ctx, tc)
}

func (h testHooks) OnToolCallEnd(ctx context.Context, tc *ToolCallContext[struct{}]) error {
	if h.ToolCallEnd == nil {
		return nil
	}
	return h.ToolCallEnd(ctx, tc)
}

func (h testHooks) OnRunStart(ctx context.Context, rc *RunContext[struct{}]) error {
	if h.RunStart == nil {
		return nil
	}
	return h.RunStart(ctx, rc)
}

func (h testHooks) OnRoundStart(ctx context.Context, rc *RoundContext[struct{}]) error {
	if h.RoundStart == nil {
		return nil
	}
	return h.RoundStart(ctx, rc)
}

func (h testHooks) OnRoundComplete(ctx context.Context, rc *RoundContext[struct{}]) error {
	if h.RoundComplete == nil {
		return nil
	}
	return h.RoundComplete(ctx, rc)
}

func (h testHooks) OnRoundEnd(ctx context.Context, rc *RoundContext[struct{}]) error {
	if h.RoundEnd == nil {
		return nil
	}
	return h.RoundEnd(ctx, rc)
}

func (h testHooks) OnRunEnd(ctx context.Context, rc *RunContext[struct{}]) {
	if h.RunEnd == nil {
		return
	}
	h.RunEnd(ctx, rc)
}
