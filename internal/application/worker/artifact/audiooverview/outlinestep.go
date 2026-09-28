package audiooverview

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
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

type podcastOutlineSegment struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type podcastOutlineExpectation struct {
	Title    string                  `json:"title"`
	Segments []podcastOutlineSegment `json:"segments"`
}

// outlineStep 生成/恢复播客大纲，写入 checkpoint.field1。
type outlineStep struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

func (s *outlineStep) Name() string { return "outline" }

func (s *outlineStep) Execute(ctx context.Context, data *pipeline.Data) error {
	req := types.RequestFrom(data)
	outline, ckpt, err := s.ensure(ctx, req, payloadFrom(req), checkpointFrom(data))
	if err != nil {
		return errors.WithMessagef(err, "generate outline failed")
	}
	data.Set(dataKeyOutline, outline)
	data.Set(dataKeyCheckpoint, ckpt)
	return nil
}

func (s *outlineStep) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	ckpt *workerentity.Checkpoint,
) (*podcastOutlineExpectation, *workerentity.Checkpoint, error) {
	if outline := s.restore(ctx, req.ArtifactId, ckpt); outline != nil {
		return outline, ckpt, nil
	}

	outline, err := s.generate(ctx, req, payload)
	if err != nil {
		return nil, ckpt, err
	}

	ckpt, err = s.save(ctx, req.ArtifactId, ckpt, outline)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "save outline checkpoint failed")
	}

	return outline, ckpt, nil
}

func (s *outlineStep) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
) (*podcastOutlineExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioAudioOverviewOutlineScene)

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderPodcastOutline(ctx, sourceIds, payload.Language, payload.GetTip(), payload.Style)
	if err != nil {
		return nil, errors.WithMessagef(err, "render podcast outline prompt failed")
	}

	ag, err := newAudioAgent(s.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*podcastOutlineExpectation](ag, "podcast outline").
		WithParse(s.parse).
		WithDuty("Produce the JSON podcast outline (title/segments) based on the given source content").
		WithRules(s.compensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func (s *outlineStep) compensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `segments`",
	}
}

func (s *outlineStep) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	outline *podcastOutlineExpectation,
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

func (s *outlineStep) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *podcastOutlineExpectation {
	if ckpt == nil || ckpt.Field1 == nil {
		return nil
	}
	var outline podcastOutlineExpectation
	if err := sonic.Unmarshal(ckpt.Field1, &outline); err != nil {
		slog.WarnContext(ctx, "unmarshal outline failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}

	return &outline
}

func (s *outlineStep) parse(ctx context.Context, content string) (*podcastOutlineExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect podcastOutlineExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"podcast outline direct unmarshal did not match, fallback to json extraction",
				slog.String("err", types.TruncateForLog(err.Error())),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "podcast outline output unmarshal failed after compatibility fallback",
			slog.String("err", types.TruncateForLog(err.Error())),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = types.NormalizeTitle(expect.Title)

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
