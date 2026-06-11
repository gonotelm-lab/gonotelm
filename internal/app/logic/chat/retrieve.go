package chat

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/bytedance/sonic"
	"github.com/gonotelm-lab/gonotelm/internal/app/agent"
	"github.com/gonotelm-lab/gonotelm/internal/app/agent/tool"
	bizsource "github.com/gonotelm-lab/gonotelm/internal/app/biz/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgslices "github.com/gonotelm-lab/gonotelm/pkg/slices"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
)

// 处理文章召回
func (l *Logic) processRetrievingSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	params *CreateUserMessageParams,
	taskId string,
) ([]*model.SourceDoc, error) {
	if len(params.SourceIds) == 0 {
		return nil, nil
	}

	if l.isTaskAborted(ctx, taskId) {
		return nil, errors.WithStack(fmt.Errorf("task %s is aborted", taskId))
	}

	query := &bizsource.CheckSourceIdsReadyQuery{
		NotebookId: notebookId,
		SourceIds:  params.SourceIds,
	}
	existSourceIds, err := l.sourceBiz.CheckSourceIdsReady(ctx, query)
	if err != nil {
		return nil, errors.WithMessage(err, "check source ids failed")
	}

	if len(existSourceIds) == 0 {
		return nil, errors.ErrParams.Msgf(
			"no source ids found, notebook_id=%s, source_ids=%v",
			notebookId, params.SourceIds)
	}

	var (
		enhancedSourceDocs []*model.SourceDoc
		intention          string
		shouldContinue     = true
	)

	// 增强检索会使用Agent召回一遍文档 再使用向量检索再召回一遍 然后将两者结果拼接
	if params.EnhancedRetrieval {
		var result *agentRetrivalResult
		result, shouldContinue, err = l.retrieveSourceDocsByAgent(
			ctx,
			notebookId,
			existSourceIds,
			params.Prompt,
			params.EnableThinking,
			taskId,
		)
		if err != nil {
			slog.ErrorContext(ctx, "retrieve source docs by agent failed",
				slog.String("task_id", taskId), slog.Any("err", err),
			)
		} else {
			intention = result.intention
			enhancedSourceDocs = result.sourceDocs
		}
	}

	if !shouldContinue {
		slog.InfoContext(ctx, "skip retrieval by agent decision",
			slog.String("task_id", taskId),
			slog.String("intention", intention),
		)
		return []*model.SourceDoc{}, nil
	}

	userPrompt := params.Prompt
	if intention != "" {
		userPrompt += " " + intention
	}
	vecSearchSourceDocs, err := l.retrieveSourceDocs(
		ctx,
		notebookId,
		userPrompt,
		existSourceIds,
		taskId,
	)
	if err != nil {
		slog.ErrorContext(ctx, "chat logic retrieve source docs failed",
			slog.String("task_id", taskId), slog.Any("err", err),
		)

		if len(enhancedSourceDocs) > 0 {
			return enhancedSourceDocs, nil
		}

		return nil, errors.WithMessage(err, "retrieve source docs failed")
	}

	// agent搜索的内容和向量检索的内容放在一起
	vecSearchSourceDocs = append(vecSearchSourceDocs, enhancedSourceDocs...)
	// 去重
	vecSearchSourceDocs = pkgslices.UniqueyFn(vecSearchSourceDocs,
		func(doc *model.SourceDoc) string {
			return doc.Id
		})

	slog.DebugContext(ctx, "successfully retrieved source docs",
		slog.String("task_id", taskId), slog.Int("count", len(vecSearchSourceDocs)),
	)

	return vecSearchSourceDocs, nil
}

// 手动召回文档
func (l *Logic) retrieveSourceDocs(
	ctx context.Context,
	notebookId uuid.UUID,
	userPrompt string,
	sourceIds []uuid.UUID,
	taskId string,
) ([]*model.SourceDoc, error) {
	retrieved, err := l.sourceBiz.SimilaritySearchSourceDocs(ctx,
		&bizsource.SimilaritySearchSourceDocsQuery{
			NotebookId: notebookId,
			SourceIds:  sourceIds,
			Query:      userPrompt,
			Count:      conf.Global().Logic.Chat.GetSourceDocsRecallCount(),
		})
	if err != nil {
		return nil, errors.WithMessage(err, "recall source docs failed")
	}

	slog.DebugContext(ctx,
		fmt.Sprintf("successfully retrieved %d source docs", len(retrieved)),
		slog.String("task_id", taskId),
		slog.String("notebook_id", notebookId.String()),
	)

	return retrieved, nil
}

type agentRetrivalResult struct {
	intention  string
	sourceDocs []*model.SourceDoc
}

// 给Agent可调用的工具 交由Agent决定召回哪些文档
func (l *Logic) retrieveSourceDocsByAgent(
	ctx context.Context,
	notebookId uuid.UUID,
	sourceIds []uuid.UUID,
	userPrompt string,
	enableThinking bool,
	taskId string,
) (*agentRetrivalResult, bool, error) {
	type state struct{}
	var (
		provider       = conf.Global().Logic.Chat.ModelProvider
		llmModel       = conf.Global().Logic.Chat.Model
		shouldContinue = true
	)

	llm, err := l.llmGateway.GetProvider(provider)
	if err != nil {
		return nil, shouldContinue, errors.WithMessage(err, "get chat llm failed")
	}

	llmOptions := []einomodel.Option{llmchat.BuildLLMModelOption(llmModel)}
	if enableThinking {
		llmOptions = append(llmOptions, llmchat.BuildThinkingOption(provider, true))
	}

	agentConfig := agent.Config[state]{
		LLM:     llm,
		Options: llmchat.BuildLLMOptions(llmOptions...),
	}

	sourcePermissionChecker := tool.SourceCheckerFn(
		func(ctx context.Context, sourceId uuid.UUID) error {
			if !slices.Contains(sourceIds, sourceId) {
				return fmt.Errorf("permission denied")
			}

			return nil
		})

	agent := agent.New(agentConfig, state{})
	// 绑定工具
	agent.BindTools(map[string]einotool.InvokableTool{
		tool.GrepSourceToolName:  tool.NewGrepSourceTool(l.sourceBizForAgent, sourcePermissionChecker),
		tool.StatSourceToolName:  tool.NewStatSourceTool(l.sourceBizForAgent, sourcePermissionChecker),
		tool.QuerySourceToolName: tool.NewQuerySourceTool(l.sourceBizForAgent, notebookId, sourcePermissionChecker),
	})

	sources, err := l.sourceBiz.BatchGetSources(ctx, notebookId, sourceIds)
	if err != nil {
		slog.ErrorContext(ctx, "batch get sources failed",
			slog.String("task_id", taskId), slog.Any("err", err),
		)
	}
	sourcesMap := make(map[uuid.UUID]*model.Source)
	for _, source := range sources {
		sourcesMap[source.Id] = source
	}

	potentialSources := make([]*prompts.RetrieveSource, 0, len(sources))
	for _, id := range sourceIds {
		var name, abstract string
		source, ok := sourcesMap[id]
		if ok {
			name = source.Title
			abstract = source.Abstract
		}

		potentialSources = append(potentialSources, &prompts.RetrieveSource{
			Id:       id.String(),
			Name:     name,
			Abstract: abstract,
		})
	}

	msg, err := prompts.RenderRetrieveSourceDocMessage(
		ctx,
		userPrompt,
		notebookId.String(),
		potentialSources,
		pkgcontext.GetLang(ctx),
	)
	if err != nil {
		return nil, shouldContinue, errors.WithMessage(err, "render retrieve source doc prompt failed")
	}

	output, err := agent.React(ctx, pkgslices.FromSingle(msg))
	if err != nil {
		return nil, shouldContinue, errors.WithMessage(err, "react retrieve source doc prompt failed")
	}

	var expect agentRetrieveSourceDocExpect
	err = sonic.Unmarshal(pkgstring.AsBytes(output.Content), &expect)
	if err != nil {
		slog.ErrorContext(ctx, "unmarshal retrieve source doc expect failed",
			slog.String("task_id", taskId), slog.Any("err", err),
			slog.String("content", string(output.Content)),
		)

		return nil, shouldContinue, errors.WithMessage(err, "unmarshal retrieve source doc expect failed")
	}

	if !expect.ShouldContinueOrDefault() {
		slog.DebugContext(ctx, "skip retrieval by expect.should_continue=false",
			slog.String("task_id", taskId),
			slog.String("intention", expect.Intention),
		)

		return &agentRetrivalResult{
			intention: expect.Intention,
		}, false, nil
	}

	if len(expect.DocIds) > 0 {
		docIds := make([]string, 0, len(expect.DocIds))
		for _, docId := range expect.DocIds {
			docIds = append(docIds, docId.String())
		}
		retrievedDocs, err := l.sourceBiz.BatchGetSourceDocs(ctx,
			&bizsource.BatchGetSourceDocsQuery{
				NotebookId: notebookId,
				DocIds:     docIds,
				Populate:   true,
			})
		if err != nil {
			return nil, shouldContinue, errors.WithMessage(err, "batch get source docs failed")
		}

		slog.DebugContext(ctx, "successfully retrieved source docs by agent",
			slog.String("task_id", taskId), slog.String("intention", expect.Intention),
			slog.Int("count", len(retrievedDocs)),
		)

		return &agentRetrivalResult{
			intention:  expect.Intention,
			sourceDocs: retrievedDocs,
		}, shouldContinue, nil
	}

	slog.DebugContext(ctx, "no source docs retrieved by agent",
		slog.String("task_id", taskId), slog.String("intention", expect.Intention),
	)

	return &agentRetrivalResult{
		intention: expect.Intention,
	}, shouldContinue, nil
}

type agentRetrieveSourceDocExpect struct {
	Intention      string      `json:"intention"`
	DocIds         []uuid.UUID `json:"doc_ids"`
	ShouldContinue *bool       `json:"should_continue,omitempty"`
}

func (e agentRetrieveSourceDocExpect) ShouldContinueOrDefault() bool {
	if e.ShouldContinue == nil {
		return true
	}

	return *e.ShouldContinue
}
