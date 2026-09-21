package types

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

type Agent = pkgagent.Agent[*SessionState]

func buildSourceExploreAgent(
	deps *WorkerDeps,
	modelProvider llmchat.Provider,
	maxRound int,
	options []einomodel.Option,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
	bindAllTools bool,
) (*Agent, error) {
	tcm, err := deps.LLMGateway.GetChatModel(modelProvider)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrParams, "get source explore llm model failed: %v", err)
	}

	agConfig := pkgagent.Config[*SessionState]{
		MaxRound: maxRound,
		BaseLLM:  tcm,
		Options:  options,
		Verbose:  conf.WorkerGlobal().Worker.AgentVerbose,
	}

	ag := pkgagent.New(agConfig, &SessionState{
		NotebookId: notebookId,
		SourceIds:  sourceIds,
	})

	spChecker := sourceCheckerFromSourceIDs(sourceIds)
	if bindAllTools {
		err = ag.BindTools(map[string]einotool.InvokableTool{
			tools.ReadSourceToolName:  tools.NewReadSourceTool(deps.Agentize, spChecker),
			tools.GrepSourceToolName:  tools.NewGrepSourceTool(deps.Agentize, spChecker),
			tools.StatSourceToolName:  tools.NewStatSourceTool(deps.Agentize, spChecker),
			tools.QuerySourceToolName: tools.NewQuerySourceTool(deps.Agentize, notebookId, spChecker),
		})
	} else {
		err = ag.BindTools(map[string]einotool.InvokableTool{
			tools.StatSourceToolName:  tools.NewStatSourceTool(deps.Agentize, spChecker),
			tools.GrepSourceToolName:  tools.NewGrepSourceTool(deps.Agentize, spChecker),
			tools.QuerySourceToolName: tools.NewQuerySourceTool(deps.Agentize, notebookId, spChecker),
		})
	}
	if err != nil {
		return nil, errors.Wrapf(errors.ErrParams, "bind source tools failed: %v", err)
	}

	ag.OnBeforeRound(pkgagent.NewFinalRoundHook(ag, maxRound))

	return ag, nil
}

func sourceCheckerFromSourceIDs(sourceIDs []valobj.Id) tools.SourcePermissionChecker {
	allowedSourceIDs := make(map[valobj.Id]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		allowedSourceIDs[sourceID] = struct{}{}
	}

	return tools.SourcePermissionCheckerFunc(func(_ context.Context, sourceIds []valobj.Id) error {
		for _, sourceId := range sourceIds {
			if _, ok := allowedSourceIDs[sourceId]; !ok {
				return fmt.Errorf("not allowed to access source: %s", sourceId)
			}
		}
		return nil
	})
}

func SourceIDsToStrings(ids []valobj.Id) []string {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	return strs
}

// ExploreAgentBuilder 逐步收集 source explore agent 的构造参数，未设置项使用默认值：
// MaxRound=conf.DefaultMaxRound、BindAllTools=true。
type ExploreAgentBuilder struct {
	deps         *WorkerDeps
	provider     llmchat.Provider
	model        string
	maxRound     int
	bindAllTools bool
	options      []einomodel.Option
}

func NewExploreAgentBuilder(deps *WorkerDeps) *ExploreAgentBuilder {
	return &ExploreAgentBuilder{deps: deps, maxRound: conf.DefaultMaxRound, bindAllTools: true}
}

func (b *ExploreAgentBuilder) WithModel(provider llmchat.Provider, model string) *ExploreAgentBuilder {
	b.provider = provider
	b.model = model
	return b
}

func (b *ExploreAgentBuilder) WithMaxRound(round int) *ExploreAgentBuilder {
	if round > 0 {
		b.maxRound = round
	}
	return b
}

// WithoutBindAllTools 关闭全量工具绑定，仅绑定 StatSource/GrepSource/QuerySource。
func (b *ExploreAgentBuilder) WithoutBindAllTools() *ExploreAgentBuilder {
	b.bindAllTools = false
	return b
}

func (b *ExploreAgentBuilder) WithOptions(options ...einomodel.Option) *ExploreAgentBuilder {
	b.options = append(b.options, options...)
	return b
}

func (b *ExploreAgentBuilder) Build(req *Request) (*Agent, error) {
	return buildSourceExploreAgent(
		b.deps,
		b.provider,
		b.maxRound,
		b.options,
		req.NotebookId,
		req.SourceIds,
		b.bindAllTools,
	)
}

// AgentStep 描述一个 agent 步骤的输出契约，并负责执行 React → 解析 → 补偿重试。
// 仅可通过 AgentStepBuilder 构造。
type AgentStep[T any] struct {
	agent       *Agent
	name        string
	duty        string
	maxRetry    int
	plainOutput bool
	rules       func(error) []string
	parse       func(context.Context, string) (T, error)
}

// AgentStepBuilder 链式收集步骤配置，未设置项使用默认值：
// Retry=1、PlainOutput=false、Rules=无额外约束、Duty=空。
type AgentStepBuilder[T any] struct {
	agent       *Agent
	name        string
	duty        string
	parse       func(context.Context, string) (T, error)
	maxRetry    int
	plainOutput bool
	rules       func(error) []string
}

func NewAgentStepBuilder[T any](agent *Agent, name string) *AgentStepBuilder[T] {
	return &AgentStepBuilder[T]{
		agent:    agent,
		name:     name,
		maxRetry: 1,
		rules:    func(error) []string { return nil },
	}
}

func (b *AgentStepBuilder[T]) WithParse(parse func(context.Context, string) (T, error)) *AgentStepBuilder[T] {
	b.parse = parse
	return b
}

func (b *AgentStepBuilder[T]) WithRetry(retry int) *AgentStepBuilder[T] {
	if retry >= 0 {
		b.maxRetry = retry
	}
	return b
}

func (b *AgentStepBuilder[T]) WithPlainOutput(plain bool) *AgentStepBuilder[T] {
	b.plainOutput = plain
	return b
}

// WithDuty sets this step's duty, injected at the top of every compensate message.
func (b *AgentStepBuilder[T]) WithDuty(duty string) *AgentStepBuilder[T] {
	b.duty = duty
	return b
}

func (b *AgentStepBuilder[T]) WithRules(rules func(error) []string) *AgentStepBuilder[T] {
	if rules != nil {
		b.rules = rules
	}
	return b
}

func (b *AgentStepBuilder[T]) Build() AgentStep[T] {
	return AgentStep[T]{
		agent:       b.agent,
		name:        b.name,
		duty:        b.duty,
		maxRetry:    b.maxRetry,
		plainOutput: b.plainOutput,
		rules:       b.rules,
		parse:       b.parse,
	}
}

func (s AgentStep[T]) Run(ctx context.Context, msgs []*einoschema.Message) (T, error) {
	var zero T
	if s.agent == nil {
		return zero, errors.Errorf("agent step %s agent is not configured", s.name)
	}
	if s.parse == nil {
		return zero, errors.Errorf("agent step %s parse is not configured", s.name)
	}

	output, err := s.agent.React(ctx, msgs)
	if err != nil {
		return zero, errors.WithMessagef(err, "generate %s output failed", s.name)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate %s agent usage: %+v", s.name, s.agent.TokenUsage()))

	parsed, err := s.parse(ctx, output.Content)
	if err == nil {
		return parsed, nil
	}

	return s.compensate(ctx, err)
}

// compensate re-prompts on parse failure until success or retries are exhausted.
// The previous output stays in the agent context (React accumulates messages), so
// the compensate message only carries the step duty and the parse error.
func (s AgentStep[T]) compensate(ctx context.Context, firstErr error) (T, error) {
	var zero T
	lastErr := firstErr

	for attempt := 1; attempt <= s.maxRetry; attempt++ {
		slog.WarnContext(ctx, "agent output invalid, compensating",
			slog.String("step", s.name),
			slog.Int("attempt", attempt),
			slog.Int("max_retry", s.maxRetry),
			slog.Any("err", lastErr),
			slog.Any("usage", s.agent.TokenUsage()),
		)

		var compensateMsg *einoschema.Message
		if s.plainOutput {
			compensateMsg = BuildCompensatePlainMessage(s.duty, s.rules(lastErr))
		} else {
			compensateMsg = BuildCompensateMessage(s.duty, s.rules(lastErr))
		}

		llmResp, genErr := s.agent.React(ctx, []*einoschema.Message{compensateMsg})
		if genErr != nil {
			return zero, errors.WithMessagef(errors.ErrLLM, "%s compensate generate failed on attempt %d, err=%v", s.name, attempt, genErr)
		}

		parsed, err := s.parse(ctx, llmResp.Content)
		if err == nil {
			return parsed, nil
		}

		lastErr = err
	}

	return zero, errors.WithMessagef(errors.ErrLLM, "%s agent output invalid after %d retries, err=%v", s.name, s.maxRetry, lastErr)
}
