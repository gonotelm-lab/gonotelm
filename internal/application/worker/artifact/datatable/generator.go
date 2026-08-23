package datatable

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const dataTableMaxCompensateRetry = 3

type Generator struct {
	deps *types.ServiceDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	tableText, err := g.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	title := g.generateTitle(ctx, tableText, req)

	return &types.Response{
		Title:      title,
		Result:     pkgstring.AsBytes(tableText),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (g *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (string, error) {
	var (
		model         = conf.WorkerGlobal().Studio.DataTable.Model
		modelProvider = conf.WorkerGlobal().Studio.DataTable.ModelProvider
		modelOption   = chat.WithModel(model)
		maxRound      = conf.WorkerGlobal().Studio.DataTable.MaxRound
		tip           = artifactentity.PayloadAs[*artifactentity.DataTablePayload](req.Payload).GetTip()
	)

	ag, err := types.BuildSourceExploreAgent(
		g.deps,
		modelProvider,
		model,
		maxRound,
		[]einomodel.Option{modelOption},
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "build source explore agent for datatable failed, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderDataTable(ctx, sourceIds, tip)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "generate datatable message failed, err=%v", err)
	}

	output, err := ag.React(ctx, msgs)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "generate datatable output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate datatable agent usage: %+v", ag.TokenUsage()))

	normalized, lastErr := NormalizeDataTableMarkdown(string(output.Content))
	if lastErr == nil {
		return normalized, nil
	}

	lastContent := string(output.Content)
	compensateMsgs := append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	for attempt := 1; attempt <= dataTableMaxCompensateRetry; attempt++ {
		slog.WarnContext(ctx, "datatable agent output invalid, compensating",
			slog.Int("attempt", attempt),
			slog.Int("max_retry", dataTableMaxCompensateRetry),
			slog.String("err", lastErr.Error()),
		)
		compensateMsgs = append(compensateMsgs, types.BuildCompensatePlainMessage(lastContent, dataTableCompensateRules(lastErr)))
		llmResp, genErr := ag.BaseLLM().Generate(ctx, compensateMsgs, modelOption)
		if genErr != nil {
			return "", errors.Wrapf(errors.ErrLLM,
				"datatable compensate generate failed on attempt %d, err=%v",
				attempt,
				genErr,
			)
		}
		lastContent = llmResp.Content
		normalized, lastErr = NormalizeDataTableMarkdown(lastContent)
		if lastErr == nil {
			return normalized, nil
		}
	}

	return "", errors.Wrapf(errors.ErrLLM,
		"datatable agent output invalid after %d retries, last_output=%q, err=%v",
		dataTableMaxCompensateRetry,
		lastContent,
		lastErr,
	)
}

func dataTableCompensateRules(validateErr error) []string {
	rules := []string{
		"Output only one GFM Markdown pipe table, with no explanatory text",
		"Do not wrap the table in ``` code fences",
		"Must include a header row, a separator row (---|---), and at least one data row",
		"All rows must have the same column count; no paragraphs/headings/lists outside the table",
	}
	if validateErr != nil {
		rules = append(rules, "Previous failure reason: "+validateErr.Error())
	}
	return rules
}

func (g *Generator) generateTitle(ctx context.Context, tableText string, req *types.Request) string {
	_ = req
	title := ""
	titleMakerMsgs, err := RenderTitleMaker(ctx, tableText)
	if err != nil {
		slog.ErrorContext(ctx, "generate title maker message failed", slog.Any("err", err))
	} else {
		modelOption := chat.WithModel(conf.WorkerGlobal().Studio.DataTable.Model)
		llmModel, llmErr := g.deps.LLMGateway.GetProvider(conf.WorkerGlobal().Studio.DataTable.ModelProvider)
		if llmErr != nil {
			slog.ErrorContext(ctx, "get llm provider for title generation failed", slog.Any("err", llmErr))
		} else {
			result, genErr := llmModel.Generate(ctx, titleMakerMsgs, modelOption)
			if genErr == nil {
				title = strings.TrimSpace(result.Content)
			} else {
				slog.ErrorContext(ctx, "generate title failed", slog.Any("err", genErr))
			}
		}
		if title == "" {
			idx := strings.Index(tableText, "\n")
			if idx > 0 {
				title = strings.TrimSpace(tableText[:idx])
			} else {
				title = strings.TrimSpace(tableText)
			}
		}
	}

	return types.NormalizeTitle(title)
}
