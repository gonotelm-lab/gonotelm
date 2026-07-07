package studio

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	bizprompt "github.com/gonotelm-lab/gonotelm/internal/app/biz/prompt"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/bytedance/sonic"
	einomodel "github.com/cloudwego/eino/components/model"
)

type audioOverviewGenerator struct {
	l *Logic
}

var (
	_ taskHandler       = &audioOverviewGenerator{}
	_ iCommonTaskParams = &generateAudioOverviewTaskParams{}
)

type generateAudioOverviewTaskParams struct {
	*commonTaskParams
	*AudioOverviewExtrasParams
}

type AudioOverviewExtrasParams struct {
	Tip      string                           `json:"tip"`
	Language string                           `json:"language"`
	Style    model.ArtifactAudioOverviewStyle `json:"style"`
}

func (p *AudioOverviewExtrasParams) GetTip() string {
	if p != nil {
		return p.Tip
	}

	return ""
}

func (p *AudioOverviewExtrasParams) GetLanguage() string {
	if p != nil {
		return p.Language
	}

	return "zh-cn(简体中文)"
}

func (p *AudioOverviewExtrasParams) GetStyle() model.ArtifactAudioOverviewStyle {
	if p != nil {
		return p.Style
	}

	return model.ArtifactAudioOverviewStyleAbstract
}

func (ag *audioOverviewGenerator) handle(
	ctx context.Context,
	task *model.ArtifactTask,
) (*taskHandleResult, error) {
	var params generateAudioOverviewTaskParams
	err := sonic.Unmarshal(task.Payload, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal generate audio overview task params err=%v", err)
	}

	expect, err := ag.generate(ctx, task.Id, &params)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate audio overview failed, err=%v", err)
	}

	_ = expect
	return nil, nil
}

type audioOverviewExpectation struct {
	Title string `json:"title"`
}

func (ag *audioOverviewGenerator) generate(
	ctx context.Context,
	taskId uuid.UUID,
	params *generateAudioOverviewTaskParams,
) (*audioOverviewExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioAudioOverviewScene)
	expect, err := ag.generateTitleAndOutline(ctx, params)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "generate audio overview done",
		slog.String("task_id", taskId.String()),
		slog.String("title", expect.Title),
	)

	return nil, fmt.Errorf("not implemented")
}

type titleAndOutlineExpectation struct {
	Title    string           `json:"title"`
	Segments []outlineSegment `json:"segments"`
}

type outlineSegment struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

func (ag *audioOverviewGenerator) generateTitleAndOutline(
	ctx context.Context,
	params *generateAudioOverviewTaskParams,
) (*titleAndOutlineExpectation, error) {
	cfg := conf.Global().Logic.Studio.AudioOverview
	modelOption := llm.WithModel(cfg.Model)

	agent, err := ag.l.buildSourceExploreAgent(
		cfg.ModelProvider,
		cfg.Model,
		cfg.MaxRound,
		[]einomodel.Option{modelOption},
		params,
		true,
	)
	if err != nil {
		return nil, errors.WithMessage(err, "build source explore agent failed")
	}

	msgs, err := ag.l.prompt.RenderStudioPodcastOutlineMessage(
		ctx,
		sourceIDsToStrings(params.SourceIds),
		params.GetLanguage(),
		params.GetTip(),
		bizprompt.PodcastStyle(params.GetStyle()),
	)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "render podcast outline prompt failed, err=%v", err)
	}

	resp, err := agent.React(ctx, msgs)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "generate podcast outline failed, err=%v", err)
	}

	var expect titleAndOutlineExpectation
	err = pkgjson.Unmarshal([]byte(resp.Content), &expect)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrInner, "unmarshal podcast outline failed, err=%v", err)
	}

	slog.DebugContext(ctx,
		"generate podcast outline done",
		slog.String("content", resp.Content),
	)

	return &expect, nil
}
