package studio

import (
	"context"
	"fmt"

	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/agent"
	"github.com/gonotelm-lab/gonotelm/internal/app/agent/tool"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

const agentFinalRoundInstruction = "IMPORTANT: 这轮输出是你最后一轮输出，请直接输出最终结果，**不需要再进行工具调用**，按照你已有的信息输出最终结果"

func newFinalRoundHook[T any](
	ag *agent.Agent[T],
	maxRound int,
) agent.BeforeRoundHook[T] {
	return func(
		_ context.Context,
		round int,
		_ T,
		msgs []*einoschema.Message,
	) ([]*einoschema.Message, error) {
		if round >= maxRound-1 {
			msgs = append(msgs, &einoschema.Message{
				Role:    einoschema.User,
				Content: agentFinalRoundInstruction,
			})
			ag.StripTools()
		}

		return msgs, nil
	}
}

func bindAgentSourceTools[T any](
	ag *agent.Agent[T],
	sbz *bizsource.AgentBiz,
	notebookId uuid.UUID,
	checker tool.SourceChecker,
) error {
	return ag.BindTools(map[string]einotool.InvokableTool{
		tool.ReadSourceToolName:  tool.NewReadSourceTool(sbz, checker),
		tool.GrepSourceToolName:  tool.NewGrepSourceTool(sbz, checker),
		tool.StatSourceToolName:  tool.NewStatSourceTool(sbz, checker),
		tool.QuerySourceToolName: tool.NewQuerySourceTool(sbz, notebookId, checker),
	})
}

func bindAgentSimpleSourceTools[T any](
	ag *agent.Agent[T],
	sbz *bizsource.AgentBiz,
	notebookId uuid.UUID,
	checker tool.SourceChecker,
) error {
	return ag.BindTools(map[string]einotool.InvokableTool{
		tool.StatSourceToolName: tool.NewStatSourceTool(sbz, checker),
		tool.GrepSourceToolName: tool.NewGrepSourceTool(sbz, checker),
	})
}

func sourceCheckerFromSourceIDs(sourceIDs []uuid.UUID) tool.SourceChecker {
	allowedSourceIDs := make(map[uuid.UUID]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		allowedSourceIDs[sourceID] = struct{}{}
	}

	return tool.SourceCheckerFn(func(_ context.Context, sourceID uuid.UUID) error {
		if _, ok := allowedSourceIDs[sourceID]; !ok {
			return fmt.Errorf("not allowed to access source: %s", sourceID)
		}

		return nil
	})
}

func decodedSourcesToSourceIDs(sources []*model.DecodedSource) []uuid.UUID {
	sourceIDs := make([]uuid.UUID, 0, len(sources))
	for _, source := range sources {
		if source == nil {
			continue
		}
		sourceIDs = append(sourceIDs, source.Id)
	}

	return sourceIDs
}

func sourceIDsToStrings(sourceIDs []uuid.UUID) []string {
	ids := make([]string, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		ids = append(ids, sourceID.String())
	}

	return ids
}

func (l *Logic) buildSourceExploreAgent(
	provider chat.Provider,
	modelName string,
	maxRound int,
	options []einomodel.Option,
	params iCommonTaskParams,
	bindAllTools bool,
) (*agent.Agent[dummayState], error) {
	llmModel, err := l.llmGateway.GetProvider(provider)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "get source explore llm model failed: %v", err)
	}

	agConfig := agent.Config[dummayState]{
		MaxRound: maxRound,
		BaseLLM:  llmModel,
		Options:  options,
	}

	ag := agent.New(agConfig, dummayState{})
	spChecker := sourceCheckerFromSourceIDs(params.getSourceIds())
	if bindAllTools {
		err = bindAgentSourceTools(
			ag,
			l.sourceBizForAgent,
			params.getNotebookId(),
			spChecker,
		)
	} else {
		err = bindAgentSimpleSourceTools(
			ag,
			l.sourceBizForAgent,
			params.getNotebookId(),
			spChecker,
		)
	}
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "bind source tools failed: %v", err)
	}

	ag.OnBeforeRound(newFinalRoundHook(ag, maxRound))

	return ag, nil
}
