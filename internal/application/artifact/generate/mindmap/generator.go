package mindmap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
)

const MindmapMaxOnceToken = 32_000

const (
	mindmapTitleMinLen = 10
	mindmapTitleMaxLen = 30
)

type Generator struct {
	deps *types.ServiceDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

type mindmapExpectation struct {
	Title   string `json:"title"`
	Mindmap string `json:"mindmap"`
}

func (m *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	expect, err := m.agentCreateMindmap(ctx, req)
	if err != nil {
		return nil, err
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     pkgstring.AsBytes(expect.Mindmap),
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (m *Generator) llmOptions() []einomodel.Option {
	var (
		provider = conf.Global().Studio.Mindmap.ModelProvider
		model    = conf.Global().Studio.Mindmap.Model
	)
	llmOptions := []einomodel.Option{
		chat.WithModel(model),
		chat.WithResponseJsonObject(provider),
		chat.WithThinking(provider, false),
	}
	return llmOptions
}

func (m *Generator) agentCreateMindmap(
	ctx context.Context,
	req *types.Request,
) (*mindmapExpectation, error) {
	llmOptions := m.llmOptions()

	ag, err := types.BuildSourceExploreAgent(
		m.deps,
		conf.Global().Studio.Mindmap.ModelProvider,
		conf.Global().Studio.Mindmap.Model,
		conf.Global().Studio.Mindmap.MaxRound,
		llmOptions,
		req.NotebookId,
		req.SourceIds,
		true,
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "failed to build source explore agent for mindmap, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderMindmap(ctx, sourceIds)
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
	msgs = append(msgs, types.BuildCompensateMessage(output.Content, []string{
		"JSON 字段必须且仅能包含 title 和 mindmap",
		"title 长度必须为 10-30 字",
		"mindmap 必须是完整 mermaid mindmap 代码块字符串",
	}))

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

func (m *Generator) parseAgentOutput(ctx context.Context, content string) (*mindmapExpectation, error) {
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

	if !CheckStudioMindmapResult(expect.Mindmap) {
		return nil, fmt.Errorf("mindmap format invalid")
	}

	return &expect, nil
}
