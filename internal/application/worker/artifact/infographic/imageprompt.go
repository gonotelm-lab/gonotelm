package infographic

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

type infoGraphicExpectation struct {
	Title       string `json:"title"`
	ImagePrompt string `json:"image_prompt"`
}

// imagePromptGenerator 生成/恢复文生图 prompt，写入 checkpoint.field1。
type imagePromptGenerator struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

func newImagePromptGenerator(deps *types.WorkerDeps, checkpoints *types.CheckpointStore) *imagePromptGenerator {
	return &imagePromptGenerator{deps: deps, checkpoints: checkpoints}
}

func (g *imagePromptGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.InfoGraphicPayload,
	ckpt *workerentity.Checkpoint,
) (*infoGraphicExpectation, *workerentity.Checkpoint, error) {
	if expect := g.restore(ctx, req.ArtifactId, ckpt); expect != nil {
		return expect, ckpt, nil
	}

	expect, err := g.generate(ctx, req, payload)
	if err != nil {
		return nil, ckpt, err
	}

	return expect, g.save(ctx, req.ArtifactId, ckpt, expect), nil
}

func (g *imagePromptGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.InfoGraphicPayload,
) (*infoGraphicExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioInfoGraphicPromptScene)

	vars := TemplateVars{
		SourceIds:    types.SourceIDsToStrings(req.SourceIds),
		TextLanguage: payload.TextLanguage,
		ExtraPrompt:  payload.ExtraPrompt,
		Orientation:  payload.Orientation,
		DetailLevel:  payload.DetailLevel,
		VisualStyle:  payload.VisualStyle,
	}
	msgs, err := RenderInfographic(ctx, vars)
	if err != nil {
		return nil, errors.WithMessagef(err, "render infographic prompt failed")
	}

	ag, err := newInfoGraphicAgent(g.deps, req, payload.DetailLevel != artifactentity.InfoGraphicDetailLevelConcise)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*infoGraphicExpectation](ag, "infographic").
		WithParse(g.parse).
		WithRules(infoGraphicCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func infoGraphicCompensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `image_prompt`",
		"`title` length must be 10-30 characters",
		"`image_prompt` must be a complete text-to-image prompt string",
	}
}

func (g *imagePromptGenerator) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	expect *infoGraphicExpectation,
) *workerentity.Checkpoint {
	promptBytes, err := sonic.Marshal(expect)
	if err != nil {
		slog.WarnContext(ctx, "marshal infographic prompt failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return ckpt
	}
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField1(promptBytes)
	if err := g.checkpoints.Save(ctx, ckpt); err != nil {
		slog.WarnContext(ctx, "save checkpoint failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
	}
	return ckpt
}

func (g *imagePromptGenerator) restore(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
) *infoGraphicExpectation {
	if ckpt == nil || ckpt.Field1 == nil {
		return nil
	}
	var expect infoGraphicExpectation
	if err := sonic.Unmarshal(ckpt.Field1, &expect); err != nil {
		slog.WarnContext(ctx, "unmarshal checkpoint prompt failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}
	return &expect
}

func (g *imagePromptGenerator) parse(
	ctx context.Context,
	content string,
) (*infoGraphicExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect infoGraphicExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"infographic direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "infographic output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)
	expect.ImagePrompt = strings.TrimSpace(expect.ImagePrompt)
	if expect.ImagePrompt == "" {
		return nil, fmt.Errorf("image_prompt is empty")
	}

	return &expect, nil
}
