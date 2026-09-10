package videooverview

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
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const (
	maxVideoSegments      = 8
	outlineCompensateRnds = 3
)

type videoOutlineSegment struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type videoOutline struct {
	Title    string                `json:"title"`
	Segments []videoOutlineSegment `json:"segments"`
}

// outlineGenerator 生成/恢复视频叙事大纲，写入 checkpoint.field1。
type outlineGenerator struct {
	deps        *types.WorkerDeps
	checkpoints *checkpointStore
}

func newOutlineGenerator(deps *types.WorkerDeps, checkpoints *checkpointStore) *outlineGenerator {
	return &outlineGenerator{deps: deps, checkpoints: checkpoints}
}

func (g *outlineGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	ckpt *workerentity.Checkpoint,
) (*videoOutline, *workerentity.Checkpoint, bool, error) {
	if outline := g.restore(ctx, req.ArtifactId, ckpt); outline != nil {
		slog.InfoContext(ctx, "video outline restored from checkpoint", slog.String("artifact_id", req.ArtifactId.String()))
		return outline, ckpt, true, nil
	}

	outline, err := g.generate(ctx, req, payload)
	if err != nil {
		return nil, ckpt, false, err
	}

	ckpt, err = g.save(ctx, req.ArtifactId, ckpt, outline)
	if err != nil {
		return nil, nil, false, errors.WithMessagef(err, "save video outline checkpoint failed")
	}

	return outline, ckpt, false, nil
}

// generate agent 探索来源产出大纲，解析失败时补偿重试。
func (g *outlineGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
) (*videoOutline, error) {
	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderVideoOutline(ctx, sourceIds, payload.GetLanguage(), payload.GetTip())
	if err != nil {
		return nil, errors.WithMessagef(err, "render video outline prompt failed")
	}

	ag, err := newVideoAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*videoOutline](ag, "video outline").
		WithParse(g.parse).
		WithRetry(outlineCompensateRnds).
		WithRules(g.compensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func (g *outlineGenerator) compensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `segments`",
		"`title` is the video title, preferably 10-30 characters",
		"`segments` is an array of 4-8 elements; each element has `name` and `content`",
	}
	if validateErr != nil {
		rules = append(rules, "Error: "+validateErr.Error())
	}
	return rules
}

func (g *outlineGenerator) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	outline *videoOutline,
) (*workerentity.Checkpoint, error) {
	data, err := sonic.Marshal(outline)
	if err != nil {
		return nil, err
	}
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField1(data)
	if err := g.checkpoints.save(ctx, ckpt); err != nil {
		return nil, err
	}
	return ckpt, nil
}

func (g *outlineGenerator) parse(ctx context.Context, content string) (*videoOutline, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var outline videoOutline
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"video outline direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &outline); err != nil {
		slog.WarnContext(ctx, "video outline output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	outline.Title = strings.TrimSpace(outline.Title)
	if outline.Title == "" {
		return nil, fmt.Errorf("video outline title is empty")
	}
	if len(outline.Segments) == 0 {
		return nil, fmt.Errorf("video outline segments is empty")
	}
	if len(outline.Segments) > maxVideoSegments {
		return nil, fmt.Errorf("video outline segments count %d exceeds max %d", len(outline.Segments), maxVideoSegments)
	}

	for i := range outline.Segments {
		seg := &outline.Segments[i]
		seg.Name = strings.TrimSpace(seg.Name)
		seg.Content = strings.TrimSpace(seg.Content)
		if seg.Name == "" {
			return nil, fmt.Errorf("segment[%d] name is empty", i)
		}
		if seg.Content == "" {
			return nil, fmt.Errorf("segment[%d] content is empty", i)
		}
	}

	return &outline, nil
}

func (g *outlineGenerator) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *videoOutline {
	if ckpt == nil || ckpt.Field1 == nil {
		return nil
	}
	var outline videoOutline
	if err := sonic.Unmarshal(ckpt.Field1, &outline); err != nil {
		slog.WarnContext(ctx, "unmarshal video outline failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}
	return &outline
}
