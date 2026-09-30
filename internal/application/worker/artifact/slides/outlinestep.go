package slides

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const slidesMaxCompensateRetry = 3

type slidesOutlineExpectation struct {
	Title   string `json:"title"`
	Outline string `json:"outline"`
}

// outlineStep 生成/恢复幻灯片大纲，写入 checkpoint.field1。
type outlineStep struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

func (s *outlineStep) Name() string { return "outline" }

func (s *outlineStep) Execute(ctx context.Context, data *pipeline.Data) error {
	outline, ckpt, err := s.ensure(ctx, types.RequestFrom(data), sourcesFrom(data), checkpointFrom(data))
	if err != nil {
		return errors.WithMessage(err, "ensure outline failed")
	}
	data.Set(dataKeyOutline, outline)
	data.Set(dataKeyCheckpoint, ckpt)
	return nil
}

func (s *outlineStep) ensure(
	ctx context.Context,
	req *types.Request,
	sources []OutlineSource,
	ckpt *workerentity.Checkpoint,
) (*slidesOutlineExpectation, *workerentity.Checkpoint, error) {
	if outline := s.restore(ctx, req.ArtifactId, ckpt); outline != nil {
		slog.InfoContext(ctx, "slides generator restore from checkpoint 1", slog.String("artifact_id", req.ArtifactId.String()))
		return outline, ckpt, nil
	}

	outline, err := s.generate(ctx, req, sources)
	if err != nil {
		return nil, ckpt, err
	}

	ckpt, err = s.save(ctx, req.ArtifactId, ckpt, outline)
	if err != nil {
		return nil, nil, errors.Wrapf(errors.ErrInner, "save slides outline checkpoint failed, err=%v", err)
	}

	return outline, ckpt, nil
}

func (s *outlineStep) generate(
	ctx context.Context,
	req *types.Request,
	sources []OutlineSource,
) (*slidesOutlineExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioSlidesOutlineScene)

	payload := artifactentity.PayloadAs[*artifactentity.SlidesPayload](req.Payload)
	msgs, err := RenderSlidesOutline(ctx, sources, payload.GetLanguage(), payload.GetTip())
	if err != nil {
		return nil, errors.WithMessagef(err, "generate slides outline message failed")
	}

	ag, err := newSlidesOutlineAgent(s.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*slidesOutlineExpectation](ag, "slides outline").
		WithParse(s.parse).
		WithRetry(slidesMaxCompensateRetry).
		WithDuty("Produce the JSON PPT outline (title/outline) based on the given source content").
		WithRules(slidesOutlineCompensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func slidesOutlineCompensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `outline`",
	}
	if validateErr != nil {
		rules = append(rules, "Error: "+validateErr.Error())
	}
	return rules
}

func (s *outlineStep) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	outline *slidesOutlineExpectation,
) (*workerentity.Checkpoint, error) {
	data, err := sonic.Marshal(outline)
	if err != nil {
		return nil, err
	}
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField1(data)

	if err := s.checkpoints.Save(ctx, ckpt); err != nil {
		return nil, err
	}
	return ckpt, nil
}

func (s *outlineStep) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *slidesOutlineExpectation {
	if ckpt == nil || ckpt.Field1 == nil {
		return nil
	}
	var outline slidesOutlineExpectation
	if err := sonic.Unmarshal(ckpt.Field1, &outline); err != nil {
		slog.WarnContext(ctx, "unmarshal slides outline failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}

	return &outline
}

func (s *outlineStep) parse(ctx context.Context, content string) (*slidesOutlineExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect slidesOutlineExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx, "slides outline direct unmarshal did not match, fallback to json extraction",
				slog.String("err", types.TruncateForLog(err.Error())))
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "slides outline output unmarshal failed after compatibility fallback",
			slog.String("err", types.TruncateForLog(err.Error())),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	if expect.Title == "" {
		return nil, fmt.Errorf("slides outline title is empty")
	}
	if expect.Outline == "" {
		return nil, fmt.Errorf("slides outline outline is empty")
	}

	return &expect, nil
}
