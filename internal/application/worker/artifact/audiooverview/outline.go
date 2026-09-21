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

// outlineGenerator 生成/恢复播客大纲，写入 checkpoint.field1。
type outlineGenerator struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

func newOutlineGenerator(deps *types.WorkerDeps, checkpoints *types.CheckpointStore) *outlineGenerator {
	return &outlineGenerator{deps: deps, checkpoints: checkpoints}
}

func (g *outlineGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	ckpt *workerentity.Checkpoint,
) (*podcastOutlineExpectation, *workerentity.Checkpoint, error) {
	if outline := g.restore(ctx, req.ArtifactId, ckpt); outline != nil {
		return outline, ckpt, nil
	}

	outline, err := g.generate(ctx, req, payload)
	if err != nil {
		return nil, ckpt, err
	}

	ckpt, err = g.save(ctx, req.ArtifactId, ckpt, outline)
	if err != nil {
		return nil, nil, errors.WithMessagef(err, "save outline checkpoint failed")
	}

	return outline, ckpt, nil
}

func (g *outlineGenerator) generate(
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

	ag, err := newAudioAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*podcastOutlineExpectation](ag, "podcast outline").
		WithParse(g.parse).
		WithDuty("Produce the JSON podcast outline (title/segments) based on the given source content").
		WithRules(g.compensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func (g *outlineGenerator) compensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `segments`",
	}
}

func (g *outlineGenerator) save(
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
	if err := g.checkpoints.Save(ctx, ckpt); err != nil {
		return nil, err
	}
	return ckpt, nil
}

func (g *outlineGenerator) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *podcastOutlineExpectation {
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

func (g *outlineGenerator) parse(ctx context.Context, content string) (*podcastOutlineExpectation, error) {
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
