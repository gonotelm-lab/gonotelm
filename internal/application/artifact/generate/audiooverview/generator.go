package audiooverview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/artifact/generate/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/bytedance/sonic"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	pkgagent "github.com/gonotelm-lab/gonotelm/pkg/agent"
)

const MaxPodcastTitleLength = 128

type Generator struct {
	deps *types.ServiceDeps
}

var _ types.Generator = &Generator{}

func New(deps *types.ServiceDeps) *Generator {
	return &Generator{deps: deps}
}

type podcastOutlineSegment struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type podcastOutlineExpectation struct {
	Title    string                  `json:"title"`
	Segments []podcastOutlineSegment `json:"segments"`
}

type podcastTranscriptTurn struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type podcastTranscriptSegment struct {
	Name     string                  `json:"name"`
	Dialogue []podcastTranscriptTurn `json:"dialogue"`
}

type podcastTranscriptExpectation struct {
	Title    string                     `json:"title"`
	Segments []podcastTranscriptSegment `json:"segments"`
}

func (a *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	payload, ok := req.Payload.(*artifactentity.AudioOverviewPayload)
	if !ok {
		return nil, errors.ErrParams.Msgf("audio overview generator expects AudioOverviewPayload")
	}

	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioAudioOverviewScene)

	llmOptions := a.llmOptions()

	outline, err := a.generateOutline(ctx, req, payload, llmOptions)
	if err != nil {
		return nil, err
	}

	transcript, err := a.generateTranscript(ctx, req, payload, outline, llmOptions)
	if err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(transcript)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal podcast transcript err=%v", err)
	}

	return &types.Response{
		Title:      transcript.Title,
		Result:     result,
		ResultKind: artifactentity.ResultKindInline,
	}, nil
}

func (a *Generator) llmOptions() []einomodel.Option {
	var (
		provider = conf.Global().Studio.AudioOverview.ModelProvider
		model    = conf.Global().Studio.AudioOverview.Model
	)
	return []einomodel.Option{
		chat.WithModel(model),
		chat.WithResponseJsonObject(provider),
		chat.WithThinking(provider, false),
	}
}

func (a *Generator) buildAgent(req *types.Request) (*pkgagent.Agent[*types.SessionState], error) {
	return types.BuildSourceExploreAgent(
		a.deps,
		conf.Global().Studio.AudioOverview.ModelProvider,
		conf.Global().Studio.AudioOverview.Model,
		conf.Global().Studio.AudioOverview.MaxRound,
		a.llmOptions(),
		req.NotebookId,
		req.SourceIds,
		true,
	)
}

func (a *Generator) generateOutline(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	llmOptions []einomodel.Option,
) (*podcastOutlineExpectation, error) {
	ag, err := a.buildAgent(req)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "build outline agent failed, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderPodcastOutline(ctx, sourceIds, payload.Language, payload.Tip, payload.Style)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "render podcast outline prompt failed, err=%v", err)
	}

	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate podcast outline output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate podcast outline agent usage: %+v", ag.TokenUsage()))

	expect, err := a.parseOutlineOutput(ctx, output.Content)
	if err == nil {
		return expect, nil
	}

	slog.WarnContext(ctx, "podcast outline agent output invalid, compensating",
		slog.String("notebook_id", req.NotebookId.String()),
		slog.Any("usage", ag.TokenUsage()),
		slog.Any("err", err),
	)

	compensateMsgs := append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	compensateMsgs = append(compensateMsgs, types.BuildCompensateMessage(output.Content, []string{
		"JSON 字段必须且仅能包含 title 和 segments",
		"title 简短精炼",
		"segments 是数组，每个元素包含 name 和 content 字段",
	}))

	llmResp, genErr := ag.BaseLLM().Generate(ctx, compensateMsgs, llmOptions...)
	if genErr != nil {
		return nil, errors.Wrapf(errors.ErrLLM,
			"podcast outline compensate generate failed, err=%v",
			genErr,
		)
	}

	expect, err = a.parseOutlineOutput(ctx, llmResp.Content)
	if err == nil {
		return expect, nil
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"podcast outline agent output invalid after compensation, first_output=%q, compensate_output=%q, err=%v",
		output.Content,
		llmResp.Content,
		err,
	)
}

func (a *Generator) generateTranscript(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	outline *podcastOutlineExpectation,
	llmOptions []einomodel.Option,
) (*podcastTranscriptExpectation, error) {
	ag, err := a.buildAgent(req)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "build transcript agent failed, err=%v", err)
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderPodcastTranscript(ctx, sourceIds, payload.Language, payload.Tip, payload.Style, outline)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "render podcast transcript prompt failed, err=%v", err)
	}

	output, err := ag.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate podcast transcript output failed, err=%v", err)
	}

	slog.InfoContext(ctx, fmt.Sprintf("generate podcast transcript agent usage: %+v", ag.TokenUsage()))

	expect, err := a.parseTranscriptOutput(ctx, output.Content, outline)
	if err == nil {
		return expect, nil
	}

	slog.WarnContext(ctx, "podcast transcript agent output invalid, compensating",
		slog.String("notebook_id", req.NotebookId.String()),
		slog.Any("usage", ag.TokenUsage()),
		slog.Any("err", err),
	)

	compensateMsgs := append([]*einoschema.Message{}, ag.AccumulatedMessages()...)
	compensateMsgs = append(compensateMsgs, types.BuildCompensateMessage(output.Content, []string{
		"JSON 字段必须且仅能包含 title 和 segments",
		"title 与大纲标题一致",
		"segments 数量与大纲一致，每个元素包含 name 和 dialogue 字段",
		"dialogue 是数组，每个元素包含 speaker 和 text 字段",
	}))

	llmResp, genErr := ag.BaseLLM().Generate(ctx, compensateMsgs, llmOptions...)
	if genErr != nil {
		return nil, errors.Wrapf(errors.ErrLLM,
			"podcast transcript compensate generate failed, err=%v",
			genErr,
		)
	}

	expect, err = a.parseTranscriptOutput(ctx, llmResp.Content, outline)
	if err == nil {
		return expect, nil
	}

	return nil, errors.Wrapf(errors.ErrLLM,
		"podcast transcript agent output invalid after compensation, first_output=%q, compensate_output=%q, err=%v",
		output.Content,
		llmResp.Content,
		err,
	)
}

func (a *Generator) parseOutlineOutput(ctx context.Context, content string) (*podcastOutlineExpectation, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect podcastOutlineExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.WarnContext(ctx,
				"podcast outline direct output unmarshal failed, fallback to extracted json candidates",
				slog.Any("err", err),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "podcast outline output unmarshal failed after compatibility fallback",
			slog.Any("err", err))
		return nil, err
	}

	expect.Title = strings.TrimSpace(expect.Title)
	expect.Title = pkgstring.TruncateRune(expect.Title, MaxPodcastTitleLength)

	if expect.Title == "" {
		return nil, fmt.Errorf("podcast outline title is empty")
	}
	if len(expect.Segments) == 0 {
		return nil, fmt.Errorf("podcast outline segments is empty")
	}

	for i := range expect.Segments {
		expect.Segments[i].Name = strings.TrimSpace(expect.Segments[i].Name)
		expect.Segments[i].Content = strings.TrimSpace(expect.Segments[i].Content)
		if expect.Segments[i].Name == "" {
			return nil, fmt.Errorf("segment[%d] name is empty", i)
		}
		if expect.Segments[i].Content == "" {
			return nil, fmt.Errorf("segment[%d] content is empty", i)
		}
	}

	return &expect, nil
}

func (a *Generator) parseTranscriptOutput(
	ctx context.Context,
	content string,
	outline *podcastOutlineExpectation,
) (*podcastTranscriptExpectation, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect podcastTranscriptExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.WarnContext(ctx,
				"podcast transcript direct output unmarshal failed, fallback to extracted json candidates",
				slog.Any("err", err),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "podcast transcript output unmarshal failed after compatibility fallback",
			slog.Any("err", err))
		return nil, err
	}

	expect.Title = strings.TrimSpace(expect.Title)

	if expect.Title == "" {
		return nil, fmt.Errorf("podcast transcript title is empty")
	}
	if len(expect.Segments) == 0 {
		return nil, fmt.Errorf("podcast transcript segments is empty")
	}
	if len(expect.Segments) != len(outline.Segments) {
		return nil, fmt.Errorf("transcript segments count %d != outline segments count %d",
			len(expect.Segments), len(outline.Segments))
	}

	for i := range expect.Segments {
		expect.Segments[i].Name = strings.TrimSpace(expect.Segments[i].Name)
		if expect.Segments[i].Name == "" {
			return nil, fmt.Errorf("transcript segment[%d] name is empty", i)
		}
		if len(expect.Segments[i].Dialogue) == 0 {
			return nil, fmt.Errorf("transcript segment[%d] dialogue is empty", i)
		}
		for j := range expect.Segments[i].Dialogue {
			expect.Segments[i].Dialogue[j].Speaker = strings.TrimSpace(expect.Segments[i].Dialogue[j].Speaker)
			expect.Segments[i].Dialogue[j].Text = strings.TrimSpace(expect.Segments[i].Dialogue[j].Text)
			if expect.Segments[i].Dialogue[j].Speaker == "" {
				return nil, fmt.Errorf("segment[%d] dialogue[%d] speaker is empty", i, j)
			}
			if expect.Segments[i].Dialogue[j].Text == "" {
				return nil, fmt.Errorf("segment[%d] dialogue[%d] text is empty", i, j)
			}
		}
	}

	return &expect, nil
}
