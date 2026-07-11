package generate

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/application/chat/agent/tools"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

const agentFinalRoundInstruction = "IMPORTANT: 这轮输出是你最后一轮输出，请直接输出最终结果，**不需要再进行工具调用**，按照你已有的信息输出最终结果"

func newFinalRoundHook[T any](
	ag *pkgagent.Agent[T],
	maxRound int,
) pkgagent.BeforeRoundHook[T] {
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

func buildSourceExploreAgent(
	deps *ServiceDeps,
	modelProvider llm.Provider,
	model string,
	maxRound int,
	options []einomodel.Option,
	notebookId valobj.Id,
	sourceIds []valobj.Id,
	bindAllTools bool,
) (*pkgagent.Agent[*SessionState], error) {
	llmModel, err := deps.LLMGateway.GetProvider(modelProvider)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "get source explore llm model failed: %v", err)
	}

	agConfig := pkgagent.Config[*SessionState]{
		MaxRound: maxRound,
		BaseLLM:  llmModel,
		Options:  options,
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
		return nil, errors.Wrapf(errors.ErrInner, "bind source tools failed: %v", err)
	}

	ag.OnBeforeRound(newFinalRoundHook(ag, maxRound))

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

func sourceIDsToStrings(ids []valobj.Id) []string {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	return strs
}
