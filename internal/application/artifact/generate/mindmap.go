package generate

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactprompt "github.com/gonotelm-lab/gonotelm/internal/application/artifact/prompt"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const (
	mindmapTitleMinLen = 10
	mindmapTitleMaxLen = 30
)

type MindmapGenerator struct {
	deps *ServiceDeps
}

type mindmapExpectation struct {
	Title   string `json:"title"`
	Mindmap string `json:"mindmap"`
}

func (m *MindmapGenerator) Generate(ctx context.Context, req *Request) (*Response, error) {
	expect, err := m.agentCreateMindmap(ctx, req)
	if err != nil {
		return nil, err
	}

	return &Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Mindmap),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (m *MindmapGenerator) llmOptions() []einomodel.Option {
	var (
		provider = conf.Global().Logic.Studio.Mindmap.ModelProvider
		model    = conf.Global().Logic.Studio.Mindmap.Model
	)
	llmOptions := []einomodel.Option{
		chat.WithModel(model),
		chat.WithResponseJsonObject(provider),
		chat.WithThinking(provider, false),
	}
	return llmOptions
}

func (m *MindmapGenerator) agentCreateMindmap(
	ctx context.Context,
	req *Request,
) (*mindmapExpectation, error) {
	llmOptions := m.llmOptions()

	ag, err := buildSourceExploreAgent(
		m.deps,
		conf.Global().Logic.Studio.Mindmap.ModelProvider,
		conf.Global().Logic.Studio.Mindmap.Model,
		conf.Global().Logic.Studio.Mindmap.MaxRound,
		llmOptions,
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "failed to build source explore agent for mindmap, err=%v", err)
	}

	sourceIds := sourceIDsToStrings(req.SourceIds)
	msgs, err := m.deps.Prompt.RenderStudioMindmapV2Message(ctx, sourceIds, pkgcontext.GetLang(ctx))
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate mindmap message failed, err=%v", err)
	}
	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate mindmap output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate mindmap agent usage: %+v", ag.TokenUsage()))

	expect, err := m.parseAgentOutput(ctx, output.Content)
	if err == nil {
		return expect, nil
	}

	slog.WarnContext(ctx, "mindmap agent output invalid, compensating",
		slog.String("notebook_id", req.NotebookId.String()),
		slog.Any("usage", ag.TokenUsage()),
	)

	msgs = append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	compensateMsg := &einoschema.Message{
		Role: einoschema.User,
		Content: fmt.Sprintf(
			"你刚才输出的结果不符合要求，请严格重输。\n当前输出：\n%s\n\n"+
				"要求：\n"+
				"1) 只输出一个合法 JSON 对象，不要任何解释性文字\n"+
				"2) JSON 字段必须且仅能包含 title 和 mindmap\n"+
				"3) title 长度必须为 10-30 字\n"+
				"4) mindmap 必须是完整 mermaid mindmap 代码块字符串\n"+
				"5) 不允许输出 ```json 代码块包裹",
			output.Content,
		),
	}
	msgs = append(msgs, compensateMsg)

	llmResp, genErr := ag.BaseLLM().Generate(ctx, msgs, llmOptions...)
	if genErr != nil {
		return nil, errors.Wrapf(errors.ErrLLM,
			"mindmap compensate generate failed, err=%v",
			genErr,
		)
	}

	expect, err = m.parseAgentOutput(ctx, llmResp.Content)
	if err == nil {
		return expect, nil
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"mindmap agent output invalid after compensation, first_output=%q, compensate_output=%q, err=%v",
		output.Content,
		llmResp.Content,
		err,
	)
}

func (m *MindmapGenerator) parseAgentOutput(ctx context.Context, content string) (*mindmapExpectation, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect mindmapExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.WarnContext(ctx, "mindmap direct output unmarshal failed, fallback to extracted json candidates",
				slog.Any("err", err))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "mindmap output unmarshal failed after compatibility fallback",
			slog.Any("err", err))
		return nil, err
	}

	expect.Title = strings.TrimSpace(expect.Title)
	expect.Mindmap = strings.TrimSpace(expect.Mindmap)

	titleLen := utf8.RuneCountInString(expect.Title)
	if titleLen > mindmapTitleMinLen {
		expect.Title = pkgstring.TruncateRune(expect.Title, mindmapTitleMaxLen)
	}

	if !artifactprompt.CheckStudioMindmapResult(expect.Mindmap) {
		return nil, fmt.Errorf("mindmap format invalid")
	}

	return &expect, nil
}
