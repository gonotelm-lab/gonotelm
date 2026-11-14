package agent

import (
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// RunConfig 描述一次执行的全部可变因素，零值表示全部使用 Runner 默认。
type RunConfig[S any] struct {
	Hooks Hooks[S]

	// MaxRound 覆盖 Runner 的默认最大轮数。
	MaxRound int
	// Model 覆盖 Runner 的默认模型。
	Model einomodel.ToolCallingChatModel
	// Options 追加到每次模型调用。
	Options []einomodel.Option

	// ExtraTools 与 Runner 静态工具合并，重名会报错。
	ExtraTools []Tool
	// WithoutTools 为 true 时本次执行不使用任何工具。
	WithoutTools bool

	// Budget 是本次运行的进程内资源上限
	Budget *RunBudget
}

// StatelessRunConfig 是无状态 agent 的 RunConfig 简写。
type StatelessRunConfig = RunConfig[NoState]

type StopReason uint8

const (
	// StopFinal 表示模型给出了无工具调用的最终回答。
	StopFinal StopReason = iota
	// StopEarly 表示 OnRoundEnd 主动结束了本次运行。
	StopEarly
	// StopContentFilter 表示模型以 content_filter 结束。
	StopContentFilter
)

// Result 是一次运行的事实输出。
type Result struct {
	Final        *schema.Message
	Stop         StopReason
	FinishReason string
	Rounds       int
	ToolCalls    int
	Usage        schema.TokenUsage
}
