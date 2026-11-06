package agent

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	agentv2 "github.com/gonotelm-lab/gonotelm/pkg/agent/v2"

	"github.com/bytedance/sonic"
)

// chatHooks 把 agent 运行生命周期的事件转成聊天业务的回调。
//
// 阶段一（非流式探索）与阶段二（流式输出）各持有一份实例，用 pushOutput 区分
// 是否把模型增量推给客户端：阶段一只产生中间结果，不应出现在用户看到的正文里。
type chatHooks struct {
	agentv2.NoopHooks[*SessionState]

	inner *Hooks
	// pushOutput 为 false 时丢弃推理与正文增量。
	pushOutput bool
	// stopPhase1 为 true 时，阶段一在 is_final 与引用都齐备后主动收尾。
	stopPhase1 bool
}

// newChatHooks 把末轮策略与业务回调组合成一个链。
func newChatHooks(inner *Hooks, pushOutput, stopPhase1 bool) agentv2.Hooks[*SessionState] {
	return agentv2.NewChainHooks(
		agentv2.FinalRoundHook[*SessionState]{},
		&chatHooks{inner: inner, pushOutput: pushOutput, stopPhase1: stopPhase1},
	)
}

// OnRoundComplete 处理本轮的工具结果：回调阶段标记，并把本轮新增消息交给业务落库。
func (h *chatHooks) OnRoundComplete(ctx context.Context, rc *agentv2.RoundContext[*SessionState]) error {
	h.markPhase(ctx, rc.Session.State, rc.Result.ToolResults)

	if h.inner != nil && h.inner.RoundFinishedHook != nil && len(rc.Result.NewMessages) > 0 {
		h.inner.RoundFinishedHook(ctx, rc.Result.NewMessages)
	}

	return nil
}

// OnRoundEnd 决定阶段一能否提前结束，交给阶段二做流式最终输出。
func (h *chatHooks) OnRoundEnd(_ context.Context, rc *agentv2.RoundContext[*SessionState]) error {
	if h.stopPhase1 && rc.Session.State.shouldEndPhase1() {
		rc.Stop = true
	}

	return nil
}

// markPhase 从工具调用参数里取出阶段信息回调业务。
// 串行/并行模式下本轮工具都已执行完毕，这里不存在并发写 state 的问题。
func (h *chatHooks) markPhase(ctx context.Context, state *SessionState, results []agentv2.ToolResult) {
	if h.inner == nil || h.inner.PhaseMarkHook == nil {
		return
	}

	for _, result := range results {
		if result.Name != tools.MarkPhaseToolName {
			continue
		}

		var input tools.MarkPhaseToolInput
		if err := sonic.Unmarshal([]byte(result.Arguments), &input); err != nil {
			slog.WarnContext(ctx, "failed to parse mark phase tool arguments",
				slog.Any("err", err),
				slog.Any("arguments", result.Arguments),
			)
			continue
		}

		state.finalPhaseMarked = input.IsFinal
		h.inner.PhaseMarkHook(ctx, Phase{
			Summary:     input.Summary,
			Description: input.Description,
			IsFinal:     input.IsFinal,
		})
	}
}

func (h *chatHooks) OnReasoningStart(ctx context.Context, _ *agentv2.OutputContext) error {
	if h.pushOutput && h.inner != nil && h.inner.ThinkStart != nil {
		h.inner.ThinkStart(ctx)
	}

	return nil
}

func (h *chatHooks) OnReasoningDelta(ctx context.Context, oc *agentv2.OutputContext) error {
	if h.pushOutput && h.inner != nil && h.inner.ThinkDelta != nil {
		h.inner.ThinkDelta(ctx, oc.Delta)
	}

	return nil
}

func (h *chatHooks) OnReasoningEnd(ctx context.Context, _ *agentv2.OutputContext) error {
	if h.pushOutput && h.inner != nil && h.inner.ThinkEnd != nil {
		h.inner.ThinkEnd(ctx)
	}

	return nil
}

func (h *chatHooks) OnContentStart(ctx context.Context, _ *agentv2.OutputContext) error {
	if h.pushOutput && h.inner != nil && h.inner.ResponseStart != nil {
		h.inner.ResponseStart(ctx)
	}

	return nil
}

func (h *chatHooks) OnContentDelta(ctx context.Context, oc *agentv2.OutputContext) error {
	if h.pushOutput && h.inner != nil && h.inner.ResponseDelta != nil {
		h.inner.ResponseDelta(ctx, oc.Delta)
	}

	return nil
}

func (h *chatHooks) OnContentEnd(ctx context.Context, _ *agentv2.OutputContext) error {
	if h.pushOutput && h.inner != nil && h.inner.ResponseEnd != nil {
		h.inner.ResponseEnd(ctx)
	}

	return nil
}
