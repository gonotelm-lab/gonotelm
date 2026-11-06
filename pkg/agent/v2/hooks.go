package agent

import (
	"context"
	"time"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// RunInfo 是一次运行/轮次的只读上下文。
type RunInfo struct {
	Agent    string
	Round    int
	MaxRound int
}

type ToolCall struct {
	Id        string
	Name      string
	Arguments string
}

type ToolResult struct {
	Id        string
	Name      string
	Arguments string
	Result    string
	Error     error
	Duration  time.Duration
}

// RoundPlan 是 OnRoundStart 可以修改的本轮执行计划。
// 零值表示全部沿用 Runner/RunConfig 的默认解析。
type RoundPlan struct {
	// Inject 是本轮临时注入的消息，不写入 Session。
	Inject []*schema.Message
	// Tools 非 nil 时覆盖本轮工具集（仍会做校验和排序）。
	Tools []Tool
	// NoTools 为 true 时本轮不绑定任何工具，优先于 Tools。
	NoTools bool
	// ToolPolicy 非 nil 时覆盖本轮工具执行策略。
	ToolPolicy *ToolPolicy
	// Model 非 nil 时覆盖本轮模型。
	Model einomodel.ToolCallingChatModel
}

// RoundResult 是一轮结束后的完整结果。
// ToolResults 按工具调用顺序排列，不受并行完成顺序影响。
type RoundResult struct {
	Assistant   *schema.Message
	ToolResults []ToolResult
	// NewMessages 是本轮追加到 Session 的消息：assistant + tool 消息，按追加顺序。
	NewMessages []*schema.Message
}

type RunContext[S any] struct {
	Session *Session[S]
	Info    RunInfo
	Result  *Result // OnRunEnd
	Err     error   // OnRunEnd
}

type RoundContext[S any] struct {
	Session *Session[S]
	Info    RunInfo

	Plan   RoundPlan   // OnRoundStart：hook 可修改
	Result RoundResult // OnRoundComplete
	Msg    *schema.Message
	Stop   bool // OnRoundEnd：hook 可设置
}

type ToolCallContext[S any] struct {
	Session *Session[S]
	Info    RunInfo
	Call    ToolCall
	Result  ToolResult // OnToolCallEnd
}

type OutputContext struct {
	Info  RunInfo
	Delta string
}

// Hooks 是 agent 生命周期与输出事件的扩展接口。
// 业务实现只需要覆写关心的阶段，建议嵌入 NoopHooks[S]。
//
// 注意：Hooks 值不能是 typed nil；RunConfig.Hooks 为 nil 时引擎会使用
// FinalRoundHook（保持零配置下的末轮行为）。
type Hooks[S any] interface {
	OnRunStart(ctx context.Context, rc *RunContext[S]) error
	OnRunEnd(ctx context.Context, rc *RunContext[S])

	OnRoundStart(ctx context.Context, rc *RoundContext[S]) error
	OnRoundComplete(ctx context.Context, rc *RoundContext[S]) error
	OnRoundEnd(ctx context.Context, rc *RoundContext[S]) error

	// OnToolCallEnd 可能在多个工具 goroutine 中并发调用，完成顺序不确定，调用方注意并发安全
	OnToolCallStart(ctx context.Context, tc *ToolCallContext[S]) error
	OnToolCallEnd(ctx context.Context, tc *ToolCallContext[S]) error

	OnReasoningStart(ctx context.Context, oc *OutputContext) error
	OnReasoningDelta(ctx context.Context, oc *OutputContext) error
	OnReasoningEnd(ctx context.Context, oc *OutputContext) error

	OnContentStart(ctx context.Context, oc *OutputContext) error
	OnContentDelta(ctx context.Context, oc *OutputContext) error
	OnContentEnd(ctx context.Context, oc *OutputContext) error
}

// NoopHooks 提供全部阶段的默认实现，业务嵌入后只覆写关心的方法。
type NoopHooks[S any] struct{}

func (NoopHooks[S]) OnRunStart(context.Context, *RunContext[S]) error { return nil }
func (NoopHooks[S]) OnRunEnd(context.Context, *RunContext[S])         {}
func (NoopHooks[S]) OnRoundStart(context.Context, *RoundContext[S]) error {
	return nil
}
func (NoopHooks[S]) OnRoundComplete(context.Context, *RoundContext[S]) error {
	return nil
}
func (NoopHooks[S]) OnRoundEnd(context.Context, *RoundContext[S]) error { return nil }
func (NoopHooks[S]) OnToolCallStart(context.Context, *ToolCallContext[S]) error {
	return nil
}
func (NoopHooks[S]) OnToolCallEnd(context.Context, *ToolCallContext[S]) error { return nil }
func (NoopHooks[S]) OnReasoningStart(context.Context, *OutputContext) error   { return nil }
func (NoopHooks[S]) OnReasoningDelta(context.Context, *OutputContext) error   { return nil }
func (NoopHooks[S]) OnReasoningEnd(context.Context, *OutputContext) error     { return nil }
func (NoopHooks[S]) OnContentStart(context.Context, *OutputContext) error     { return nil }
func (NoopHooks[S]) OnContentDelta(context.Context, *OutputContext) error     { return nil }
func (NoopHooks[S]) OnContentEnd(context.Context, *OutputContext) error       { return nil }

// NewStatelessFinalRoundHook 构造无状态 agent 的末轮策略
func NewStatelessFinalRoundHook() FinalRoundHook[NoState] {
	return FinalRoundHook[NoState]{}
}

// FinalRoundHook 是内置的末轮策略：
// 最后一轮注入 instruction，并默认不绑定工具。
type FinalRoundHook[S any] struct {
	NoopHooks[S]

	Instruction string
	KeepTools   bool
	Disable     bool
}

func (h FinalRoundHook[S]) OnRoundStart(ctx context.Context, rc *RoundContext[S]) error {
	if h.Disable || rc.Info.Round != rc.Info.MaxRound-1 {
		return nil
	}

	instruction := h.Instruction
	if instruction == "" {
		instruction = FinalRoundInstruction
	}

	rc.Plan.Inject = append(rc.Plan.Inject, schema.UserMessage(instruction))
	if !h.KeepTools {
		rc.Plan.NoTools = true
	}
	return nil
}

func hooksOrDefault[S any](hooks Hooks[S]) Hooks[S] {
	if hooks == nil {
		return FinalRoundHook[S]{}
	}
	return hooks
}
