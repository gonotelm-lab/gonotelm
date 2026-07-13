package report

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	einomodel "github.com/cloudwego/eino/components/model"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

const MaxNotebookNameLength = 128

type Generator struct {
	deps *types.ServiceDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

func (r *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	reportText, err := r.generate(ctx, req)
	if err != nil {
		return nil, err
	}

	title := r.generateTitle(ctx, reportText, req)

	return &types.Response{
		Title:      title,
		Result:     pkgstring.AsBytes(reportText),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (r *Generator) generate(
	ctx context.Context,
	req *types.Request,
) (string, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioReportScene)

	var (
		reportModel         = conf.Global().Studio.Report.Model
		reportModelProvider = conf.Global().Studio.Report.ModelProvider
		modelOption         = chat.WithModel(reportModel)
		maxRound            = conf.Global().Studio.Report.MaxRound
	)

	ag, err := types.BuildSourceExploreAgent(
		r.deps,
		reportModelProvider,
		reportModel,
		maxRound,
		[]einomodel.Option{modelOption},
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "build source explore agent for report failed, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderReport(ctx, sourceIds)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "generate report message failed, err=%v", err)
	}

	output, err := ag.React(ctx, msgs)
	if err != nil {
		return "", errors.Wrapf(errors.ErrInner, "generate report output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate report agent usage: %+v", ag.TokenUsage()))

	return string(output.Content), nil
}

func (r *Generator) generateTitle(ctx context.Context, report string, req *types.Request) string {
	title := ""
	titleMakerMsgs, err := RenderTitleMaker(ctx, report)
	if err != nil {
		slog.ErrorContext(ctx, "generate title maker message failed", slog.Any("err", err))
	} else {
		modelOption := chat.WithModel(conf.Global().Studio.Report.Model)
		llmModel, llmErr := r.deps.LLMGateway.GetProvider(conf.Global().Studio.Report.ModelProvider)
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
			idx := strings.Index(report, "\n")
			if idx > 0 {
				title = strings.TrimSpace(report[:idx])
			}
		}
	}

	title = pkgstring.TruncateRune(title, MaxNotebookNameLength)
	title = strings.TrimSpace(title)

	return title
}
