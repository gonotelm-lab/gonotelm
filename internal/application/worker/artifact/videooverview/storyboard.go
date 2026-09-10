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
	maxVideoScenes       = 20
	maxNarrationLength   = 500
	storyboardCompensate = 3
)

type storyboardSceneVisual struct {
	Headline string   `json:"headline"`
	Points   []string `json:"points,omitempty"`
	Layout   string   `json:"layout,omitempty"`
}

type storyboardScene struct {
	Name      string                `json:"name"`
	Narration string                `json:"narration"`
	Visual    storyboardSceneVisual `json:"visual"`
}

type videoStoryboard struct {
	Title  string            `json:"title"`
	Scenes []storyboardScene `json:"scenes"`
}

// storyboardGenerator 基于大纲生成/恢复分镜脚本，写入 checkpoint.field2。
type storyboardGenerator struct {
	agents      *types.AgentFactory
	checkpoints *checkpointStore
}

func newStoryboardGenerator(agents *types.AgentFactory, checkpoints *checkpointStore) *storyboardGenerator {
	return &storyboardGenerator{agents: agents, checkpoints: checkpoints}
}

func (g *storyboardGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	outline *videoOutline,
	ckpt *workerentity.Checkpoint,
) (*videoStoryboard, *workerentity.Checkpoint, bool, error) {
	if sb := g.restore(ctx, req.ArtifactId, ckpt); sb != nil {
		slog.InfoContext(ctx, "video storyboard restored from checkpoint", slog.String("artifact_id", req.ArtifactId.String()))
		return sb, ckpt, true, nil
	}

	sb, err := g.generate(ctx, req, payload, outline)
	if err != nil {
		return nil, ckpt, false, err
	}

	ckpt, err = g.save(ctx, req.ArtifactId, ckpt, sb)
	if err != nil {
		return nil, nil, false, errors.WithMessagef(err, "save video storyboard checkpoint failed")
	}

	return sb, ckpt, false, nil
}

// generate agent 基于大纲产出分镜，解析失败时补偿重试。
func (g *storyboardGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	outline *videoOutline,
) (*videoStoryboard, error) {
	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderVideoStoryboard(ctx, sourceIds, payload.GetLanguage(), payload.GetTip(), payload.GetVisualStyle(), outline)
	if err != nil {
		return nil, errors.WithMessagef(err, "render video storyboard prompt failed")
	}

	step := types.Step[*videoStoryboard]{
		Factory:  g.agents,
		Name:     "video storyboard",
		MaxRetry: storyboardCompensate,
		Rules:    g.compensateRules,
		Parse: func(ctx context.Context, content string) (*videoStoryboard, error) {
			return g.parse(ctx, content, outline)
		},
	}
	return step.Run(ctx, req, msgs)
}

func (g *storyboardGenerator) compensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `scenes`",
		"`title` must match the outline title",
		"`scenes` must cover every outline segment (at least one scene per segment); each element has `name`, `narration` and `visual`",
		"`visual` is an object with `headline`, `points` (array) and `layout`",
		"`narration` must be spoken-style plain text in the target language, single line, no markdown symbols, emoji, urls or newlines",
	}
	if validateErr != nil {
		rules = append(rules, "Error: "+validateErr.Error())
	}
	return rules
}

func (g *storyboardGenerator) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	sb *videoStoryboard,
) (*workerentity.Checkpoint, error) {
	data, err := sonic.Marshal(sb)
	if err != nil {
		return nil, err
	}
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField2(data)
	if err := g.checkpoints.save(ctx, ckpt); err != nil {
		return nil, err
	}
	return ckpt, nil
}

func (g *storyboardGenerator) parse(ctx context.Context, content string, outline *videoOutline) (*videoStoryboard, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var sb videoStoryboard
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"video storyboard direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &sb); err != nil {
		slog.WarnContext(ctx, "video storyboard output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	sb.Title = strings.TrimSpace(sb.Title)
	if sb.Title == "" {
		return nil, fmt.Errorf("video storyboard title is empty")
	}
	// title 以大纲为准，消除两步生成间的不一致
	sb.Title = outline.Title
	if len(sb.Scenes) == 0 {
		return nil, fmt.Errorf("video storyboard scenes is empty")
	}
	if len(sb.Scenes) > maxVideoScenes {
		return nil, fmt.Errorf("video storyboard scenes count %d exceeds max %d", len(sb.Scenes), maxVideoScenes)
	}
	// 每个大纲板块至少对应一场戏
	if len(sb.Scenes) < len(outline.Segments) {
		return nil, fmt.Errorf("video storyboard scenes count %d less than outline segments count %d",
			len(sb.Scenes), len(outline.Segments))
	}

	for i := range sb.Scenes {
		scene := &sb.Scenes[i]
		scene.Name = strings.TrimSpace(scene.Name)
		// narration 送 TTS 前折叠换行与连续空白，避免合成节奏异常
		scene.Narration = strings.Join(strings.Fields(scene.Narration), " ")
		scene.Visual.Headline = strings.TrimSpace(scene.Visual.Headline)
		for j := range scene.Visual.Points {
			scene.Visual.Points[j] = strings.TrimSpace(scene.Visual.Points[j])
		}
		if scene.Name == "" {
			return nil, fmt.Errorf("scene[%d] name is empty", i)
		}
		if scene.Narration == "" {
			return nil, fmt.Errorf("scene[%d] narration is empty", i)
		}
		if len([]rune(scene.Narration)) > maxNarrationLength {
			return nil, fmt.Errorf("scene[%d] narration too long (%d runes)", i, len([]rune(scene.Narration)))
		}
	}

	return &sb, nil
}

func (g *storyboardGenerator) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *videoStoryboard {
	if ckpt == nil || ckpt.Field2 == nil {
		return nil
	}
	var sb videoStoryboard
	if err := sonic.Unmarshal(ckpt.Field2, &sb); err != nil {
		slog.WarnContext(ctx, "unmarshal video storyboard failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}
	return &sb
}
