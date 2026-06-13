package studio

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	bizagent "github.com/gonotelm-lab/gonotelm/internal/app/agent"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	eino "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

type reportGenerator struct {
	l *Logic
}

var _ taskHandler = &reportGenerator{}

type generateReportTaskParams struct {
	*commonTaskParams
}

func (m *reportGenerator) handle(
	ctx context.Context,
	task *model.ArtifactTask,
) (*taskHandleResult, error) {
	var params generateReportTaskParams
	err := sonic.Unmarshal(task.Payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal generate report task params err=%v", err)
	}
	expect, err := m.generate(ctx, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate report failed, err=%v", err)
	}

	return &taskHandleResult{
		result:     pkgstring.AsBytes(expect.Report),
		resultKind: model.ArtifactResultKindInline,
		title:      expect.Title,
	}, nil
}

type reportExpectation struct {
	Title  string `json:"title"`
	Report string `json:"report"`
}

type dummayState struct{}

func (m *reportGenerator) generate(
	ctx context.Context,
	params *generateReportTaskParams,
) (*reportExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioReportScene)
	usedModel := conf.Global().Logic.Studio.Report.Model
	llmModel, err := m.l.llmGateway.GetProvider(
		conf.Global().Logic.Studio.Report.ModelProvider,
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "get mindmap llm model failed: %v", err)
	}

	modelOption := llmchat.WithModel(usedModel)
	maxRound := conf.Global().Logic.Studio.Report.MaxRound
	agentConfig := bizagent.Config[dummayState]{
		MaxRound: maxRound,
		BaseLLM:  llmModel,
		Options:  llmchat.BuildLLMOptions(modelOption),
	}

	sbz := m.l.sourceBizForAgent
	ag := bizagent.New(agentConfig, dummayState{})
	sourceChecker := sourceCheckerFromSourceIDs(params.SourceIds)
	err = bindAgentSourceTools(ag, sbz, params.NotebookId, sourceChecker)
	ag.OnBeforeRound(newFinalRoundHook(ag, maxRound))
	if err != nil {
		return nil, errors.WithMessagef(err, "bind tools failed")
	}

	sourceIds := sourceIDsToStrings(params.SourceIds)

	msg, err := prompts.RenderStudioReportMessage(ctx, sourceIds, "")
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate report message failed, err=%v", err)
	}
	output, err := ag.React(ctx, slices.FromSingle(msg))
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate report output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate report agent usage: %+v", ag.TokenUsage()))

	expect := reportExpectation{
		Report: string(output.Content),
	}

	// generate title again
	title, err := m.generateTitle(ctx,
		llmModel,
		modelOption,
		expect.Report,
		ag.AccumulatedMessages(),
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate title failed, err=%v", err)
	}
	expect.Title = title

	return &expect, nil
}

func (m *reportGenerator) generateTitle(
	ctx context.Context,
	llmModel eino.ToolCallingChatModel,
	modelOption eino.Option,
	report string,
	previousMsgs []*einoschema.Message,
) (string, error) {
	title := ""
	titleMakerMsg, err := prompts.RenderTitleMakerMessage(ctx, report, pkgcontext.GetLang(ctx))
	if err != nil {
		slog.ErrorContext(ctx, "generate title maker message failed", slog.Any("err", err))
	} else {
		msgs := make([]*einoschema.Message, 0, 1+len(previousMsgs))
		msgs = append(msgs, previousMsgs...)
		msgs = append(msgs, titleMakerMsg)
		result, err := llmModel.Generate(ctx, msgs, modelOption)
		if err == nil {
			title = strings.TrimSpace(result.Content)
		} else {
			slog.ErrorContext(ctx, "generate title failed", slog.Any("err", err))
			// take the first sentence as title
			idx := strings.Index(report, "\n")
			if idx > 0 {
				title = strings.TrimSpace(report[:idx])
			}
		}
	}

	title = pkgstring.TruncateRune(title, constants.MaxNotebookNameLength)
	title = strings.TrimSpace(title)

	return title, nil
}
