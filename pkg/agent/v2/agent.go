package agent

import (
	"context"
	"sort"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	pkgerrors "github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const defaultMaxRound = 20

// NoState 用于不维护业务状态的 agent（如 subagent）。
// 它让 Runner 的类型参数有明确语义，避免在调用点反复出现 struct{}。
type NoState struct{}

// StatelessRunner 是 Runner[NoState] 的别名，供不维护业务状态的 agent 使用。
// 调用方不需要再写泛型参数即可表示"不使用 state"。
type StatelessRunner = Runner[NoState]

// FinalRoundInstruction 是末轮默认注入的指令。
const FinalRoundInstruction = "IMPORTANT: This is your final round. Output the final result directly based on what you already have. **Do not make any more tool calls.**"

// RunnerConfig 是 Runner 的稳定配置，构造后不再变化。
// 配置本身与 state 无关，因此不再携带类型参数。
type RunnerConfig struct {
	Name     string
	Model    einomodel.ToolCallingChatModel
	Options  []einomodel.Option
	Tools    []Tool
	MaxRound int
	Policy   ToolPolicy
	Verbose  bool
}

// Runner 是不可变执行器，构造后可并发共享。
type Runner[S any] struct {
	name     string
	model    einomodel.ToolCallingChatModel
	opts     []einomodel.Option
	tools    []*toolEntry
	maxRound int
	policy   ToolPolicy
	verbose  bool
}

func NewRunner[S any](ctx context.Context, cfg RunnerConfig) (*Runner[S], error) {
	if cfg.Model == nil {
		return nil, pkgerrors.ErrParams.Msg("agent model is required")
	}
	if cfg.MaxRound <= 0 {
		cfg.MaxRound = defaultMaxRound
	}

	tools, err := validateTools(ctx, cfg.Tools)
	if err != nil {
		return nil, err
	}

	return &Runner[S]{
		name:     cfg.Name,
		model:    cfg.Model,
		opts:     cfg.Options,
		tools:    tools,
		maxRound: cfg.MaxRound,
		policy:   cfg.Policy,
		verbose:  cfg.Verbose,
	}, nil
}

// NewStatelessRunner 构造一个不维护业务状态的 Runner。
func NewStatelessRunner(ctx context.Context, cfg RunnerConfig) (*StatelessRunner, error) {
	return NewRunner[NoState](ctx, cfg)
}

// BaseModel 返回不带工具的模型，供 subagent 等场景复用。
func (r *Runner[S]) BaseModel() einomodel.ToolCallingChatModel {
	return r.model
}

// resolveTools 计算本次轮次的默认工具集。
func (r *Runner[S]) resolveTools(
	ctx context.Context,
	cfg RunConfig[S],
) ([]*toolEntry, error) {
	entries := append([]*toolEntry(nil), r.tools...)
	if len(cfg.ExtraTools) > 0 {
		extras, err := validateTools(ctx, cfg.ExtraTools)
		if err != nil {
			return nil, err
		}

		seen := make(map[string]struct{}, len(entries)+len(extras))
		for _, entry := range entries {
			seen[entry.info.Name] = struct{}{}
		}
		for _, entry := range extras {
			if _, ok := seen[entry.info.Name]; ok {
				return nil, pkgerrors.ErrParams.Msgf("duplicated tool name: %s", entry.info.Name)
			}
			seen[entry.info.Name] = struct{}{}
			entries = append(entries, entry)
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].info.Name < entries[j].info.Name
		})
	}

	return entries, nil
}

// resolveModel 按工具集构建本次调用的模型。
func (r *Runner[S]) resolveModel(
	base einomodel.ToolCallingChatModel,
	entries []*toolEntry,
) (einomodel.ToolCallingChatModel, error) {
	if len(entries) == 0 {
		return base, nil
	}

	infos := make([]*schema.ToolInfo, 0, len(entries))
	for _, entry := range entries {
		infos = append(infos, entry.info)
	}

	model, err := base.WithTools(infos)
	if err != nil {
		return nil, pkgerrors.Wrapf(pkgerrors.ErrLLM, "bind tools failed: %v", err)
	}
	return model, nil
}
