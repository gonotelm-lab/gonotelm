package studio

import (
	"context"
	"log/slog"
	"strings"

	bizagent "github.com/gonotelm-lab/gonotelm/internal/app/biz/agent"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	studiotool "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio/tool"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/internal/app/prompts"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	llmchat "github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	eino "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
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

type reportAgentState struct{}

func (m *reportGenerator) generate(
	ctx context.Context,
	params *generateReportTaskParams,
) (*reportExpectation, error) {
	usedModel := conf.Global().Logic.Studio.Report.Model
	llmModel, err := m.l.llmGateway.GetProvider(
		conf.Global().Logic.Studio.Report.ModelProvider,
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "get mindmap llm model failed: %v", err)
	}

	modelOption := llmchat.BuildLLMModelOption(usedModel)
	maxRound := conf.Global().Logic.Studio.Report.MaxRound
	agentConfig := bizagent.AgentConfig[*reportAgentState]{
		MaxRound:    maxRound,
		LLM:         llmModel,
		Options:     llmchat.BuildLLMOptions(modelOption),
		BeforeRound: m.beforeAgentRoundHook,
	}

	agent := bizagent.New(agentConfig, &reportAgentState{})
	err = agent.BindTools(map[string]einotool.InvokableTool{
		studiotool.ReadSourceToolName: studiotool.NewReadSourceTool(m.l.sourceBizForAgent),
		studiotool.GrepSourceToolName: studiotool.NewGrepSourceTool(m.l.sourceBizForAgent),
		studiotool.StatSourceToolName: studiotool.NewStatSourceTool(m.l.sourceBizForAgent),
	})
	if err != nil {
		return nil, errors.WithMessagef(err, "bind tools failed")
	}

	sourceIds := make([]string, 0, len(params.SourceIds))
	for _, sourceId := range params.SourceIds {
		sourceIds = append(sourceIds, sourceId.String())
	}

	msg, err := prompts.StudioReportMessage(ctx, sourceIds, "")
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate report message failed, err=%v", err)
	}
	output, err := agent.React(ctx, slices.FromSingle(msg))
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate report output failed, err=%v", err)
	}

	expect := reportExpectation{
		Report: string(output.Content),
	}

	// generate title again
	title, err := m.generateTitle(ctx, llmModel, modelOption, expect.Report)
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
) (string, error) {
	title := ""
	summaryMsg, err := prompts.TitleMakerMessage(ctx, report, "")
	if err != nil {
		slog.ErrorContext(ctx, "generate title maker message failed", slog.Any("err", err))
	} else {
		result, err := llmModel.Generate(ctx, slices.FromSingle(summaryMsg), modelOption)
		if err == nil {
			title = strings.Split(result.Content, "\n")[0]
		} else {
			slog.ErrorContext(ctx, "generate title failed", slog.Any("err", err))
			// take the first sentence as title
			title = strings.Split(summaryMsg.Content, "\n")[0]
		}
	}
	title = pkgstring.TruncateRune(title, constants.MaxNotebookNameLength)
	title = strings.TrimSpace(title)

	return title, nil
}

func (m *reportGenerator) beforeAgentRoundHook(
	ctx context.Context,
	round int,
	state *reportAgentState,
	msgs []*einoschema.Message,
) ([]*einoschema.Message, error) {
	slog.DebugContext(ctx, "generating report before round hook invoked", slog.Int("round", round))

	if round >= conf.Global().Logic.Studio.Report.MaxRound-1 {
		// 注入一条msg
		msgs = append(msgs, &einoschema.Message{
			Role:    einoschema.User,
			Content: "IMPORTANT: 这轮输出是你最后一轮输出，请直接输出最终结果，不需要再进行工具调用，按照你已有的信息输出最终结果",
		})
	}

	return msgs, nil
}
