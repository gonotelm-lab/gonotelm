package types

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

type Agent = pkgagent.Agent[*SessionState]

func BuildSourceExploreAgent(
	deps *WorkerDeps,
	modelProvider llmchat.Provider,
	model string,
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
		Verbose:  false,
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
			tools.StatSourceToolName: tools.NewStatSourceTool(deps.Agentize, spChecker),
			tools.GrepSourceToolName: tools.NewGrepSourceTool(deps.Agentize, spChecker),
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

// AgentFactory 基于已解析的模型配置构造绑定了来源工具的 agent。
// 参数由调用方注入（而非内部读取配置），保证与具体产物类型解耦。
type AgentFactory struct {
	deps     *WorkerDeps
	provider llmchat.Provider
	model    string
	maxRound int
	options  []einomodel.Option
}

func NewAgentFactory(
	deps *WorkerDeps,
	provider llmchat.Provider,
	model string,
	maxRound int,
	options []einomodel.Option,
) *AgentFactory {
	return &AgentFactory{
		deps:     deps,
		provider: provider,
		model:    model,
		maxRound: maxRound,
		options:  options,
	}
}

func (f *AgentFactory) Build(req *Request) (*Agent, error) {
	ag, err := BuildSourceExploreAgent(
		f.deps,
		f.provider,
		f.model,
		f.maxRound,
		f.options,
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return nil, errors.WithMessagef(err, "build source explore agent failed")
	}
	return ag, nil
}

// Step 描述一个 agent 步骤的输出契约，并负责执行 React → 解析 → 补偿重试。
type Step[T any] struct {
	Factory  *AgentFactory
	Name     string
	MaxRetry int
	Rules    func(error) []string
	Parse    func(context.Context, string) (T, error)
}

func (s *Step[T]) Run(ctx context.Context, req *Request, msgs []*einoschema.Message) (T, error) {
	ag, err := s.Factory.Build(req)
	if err != nil {
		var zero T
		return zero, err
	}

	output, err := ag.React(ctx, msgs)
	if err != nil {
		var zero T
		return zero, errors.WithMessagef(err, "generate %s output failed", s.Name)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate %s agent usage: %+v", s.Name, ag.TokenUsage()))

	parsed, parseErr := s.Parse(ctx, output.Content)
	if parseErr == nil {
		return parsed, nil
	}

	return s.compensate(ctx, ag, output.Content, parseErr)
}

// compensate 解析失败后带着重新约束继续对话，直至解析成功或耗尽重试次数。
func (s *Step[T]) compensate(ctx context.Context, ag *Agent, firstContent string, firstErr error) (T, error) {
	lastContent, lastErr := firstContent, firstErr
	msgs := append([]*einoschema.Message{}, ag.AccumulatedMessages()...)

	for attempt := 1; attempt <= s.MaxRetry; attempt++ {
		slog.WarnContext(ctx, "agent output invalid, compensating",
			slog.String("step", s.Name),
			slog.Int("attempt", attempt),
			slog.Int("max_retry", s.MaxRetry),
			slog.Any("err", lastErr),
			slog.Any("usage", ag.TokenUsage()),
		)

		compensateMsgs := append([]*einoschema.Message{}, msgs...)
		compensateMsgs = append(compensateMsgs, BuildCompensateMessage(lastContent, s.Rules(lastErr)))

		llmResp, genErr := ag.BaseLLM().Generate(ctx, compensateMsgs, s.Factory.options...)
		if genErr != nil {
			var zero T
			return zero, errors.WithMessagef(errors.ErrLLM, "%s compensate generate failed on attempt %d, err=%v", s.Name, attempt, genErr)
		}

		parsed, parseErr := s.Parse(ctx, llmResp.Content)
		if parseErr == nil {
			return parsed, nil
		}

		lastContent = llmResp.Content
		lastErr = parseErr
		msgs = append(compensateMsgs, &einoschema.Message{
			Role:    einoschema.Assistant,
			Content: lastContent,
		})
	}

	var zero T
	return zero, errors.WithMessagef(errors.ErrLLM, "%s agent output invalid after %d retries, err=%v", s.Name, s.MaxRetry, lastErr)
}
