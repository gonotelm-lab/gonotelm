package agent

import (
	"context"
	"slices"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// NewChainHooks 把多个 hooks 按给定顺序组合成一个 Hooks。
//
// 所有阶段都按传入顺序逐个执行，后面的成员能看到前面成员对 RoundPlan / Stop 的修改；
// 返回错误的阶段遇到第一个错误即中止，错误会带上出错成员的序号。
// 无返回值的阶段（如 OnRunEnd）无条件转发给全部成员。
//
// 链不会自动带上 FinalRoundHook：需要末轮行为时把它作为一个成员显式传入。
// 传入的 nil 成员会被忽略；零参数返回一个什么都不做的链（非 nil）。
//
// 注意：OnToolCallStart / OnToolCallEnd 仍可能在多个 goroutine 中并发调用，
// 链只做转发，各成员需自行保证并发安全。
func NewChainHooks[S any](hooks ...Hooks[S]) Hooks[S] {
	filtered := slices.DeleteFunc(slices.Clone(hooks), func(h Hooks[S]) bool {
		return h == nil
	})
	return chainHooks[S]{hooks: filtered}
}

// chainHooks 是 NewChainHooks 返回的链式实现，成员在构造时已冻结。
type chainHooks[S any] struct {
	hooks []Hooks[S]
}

var _ Hooks[struct{}] = chainHooks[struct{}]{}

func (c chainHooks[S]) OnRunStart(ctx context.Context, rc *RunContext[S]) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnRunStart(ctx, rc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnRunEnd(ctx context.Context, rc *RunContext[S]) {
	for _, h := range c.hooks {
		if h == nil {
			continue
		}
		h.OnRunEnd(ctx, rc)
	}
}

func (c chainHooks[S]) OnRoundStart(ctx context.Context, rc *RoundContext[S]) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnRoundStart(ctx, rc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnRoundComplete(ctx context.Context, rc *RoundContext[S]) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnRoundComplete(ctx, rc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnRoundEnd(ctx context.Context, rc *RoundContext[S]) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnRoundEnd(ctx, rc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnToolCallStart(ctx context.Context, tc *ToolCallContext[S]) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnToolCallStart(ctx, tc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnToolCallEnd(ctx context.Context, tc *ToolCallContext[S]) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnToolCallEnd(ctx, tc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnReasoningStart(ctx context.Context, oc *OutputContext) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnReasoningStart(ctx, oc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnReasoningDelta(ctx context.Context, oc *OutputContext) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnReasoningDelta(ctx, oc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnReasoningEnd(ctx context.Context, oc *OutputContext) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnReasoningEnd(ctx, oc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnContentStart(ctx context.Context, oc *OutputContext) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnContentStart(ctx, oc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnContentDelta(ctx context.Context, oc *OutputContext) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnContentDelta(ctx, oc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

func (c chainHooks[S]) OnContentEnd(ctx context.Context, oc *OutputContext) error {
	for i, h := range c.hooks {
		if h == nil {
			continue
		}
		if err := h.OnContentEnd(ctx, oc); err != nil {
			return chainHookError(i, err)
		}
	}

	return nil
}

// chainHookError 标出链上出错的成员序号，便于定位。
func chainHookError(index int, err error) error {
	return pkgerrors.WithMessagef(err, "chain hook %d failed", index)
}
