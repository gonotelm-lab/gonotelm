package generate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	einomodel "github.com/cloudwego/eino/components/model"
	artifactprompt "github.com/gonotelm-lab/gonotelm/internal/application/artifact/prompt"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
)

type ReportGenerator struct {
	deps *ServiceDeps
}

func (r *ReportGenerator) Generate(ctx context.Context, req *Request) (*Response, error) {
	reportText, err := r.agentCreateReport(ctx, req)
	if err != nil {
		return nil, err
	}

	title := r.generateTitle(ctx, reportText, req)

	return &Response{
		Title:      title,
		Result:     pkgstring.AsBytes(reportText),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (r *ReportGenerator) agentCreateReport(
	ctx context.Context,
	req *Request,
) (string, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioReportScene)

	var (
		reportModel         = conf.Global().Logic.Studio.Report.Model
		reportModelProvider = conf.Global().Logic.Studio.Report.ModelProvider
		modelOption         = chat.WithModel(reportModel)
		maxRound            = conf.Global().Logic.Studio.Report.MaxRound
	)

	ag, err := buildSourceExploreAgent(
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

	sourceIds := sourceIDsToStrings(req.SourceIds)
	msgs, err := artifactprompt.RenderReport(ctx, sourceIds)
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

func (r *ReportGenerator) generateTitle(ctx context.Context, report string, req *Request) string {
	title := ""
	titleMakerMsgs, err := artifactprompt.RenderTitleMaker(ctx, report)
	if err != nil {
		slog.ErrorContext(ctx, "generate title maker message failed", slog.Any("err", err))
	} else {
		modelOption := chat.WithModel(conf.Global().Logic.Studio.Report.Model)
		llmModel, llmErr := r.deps.LLMGateway.GetProvider(conf.Global().Logic.Studio.Report.ModelProvider)
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
