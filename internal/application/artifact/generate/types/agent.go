package types

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
)

func BuildSourceExploreAgent(
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
