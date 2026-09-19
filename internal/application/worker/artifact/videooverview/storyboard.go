package videooverview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

// storyboardGenerator 生成分镜 Markdown：模型读写固定草稿文件，成功后写入 checkpoint.field3。
type storyboardGenerator struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

func newStoryboardGenerator(deps *types.WorkerDeps, checkpoints *types.CheckpointStore) *storyboardGenerator {
	return &storyboardGenerator{deps: deps, checkpoints: checkpoints}
}

func (g *storyboardGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	script *videoScript,
	audioMeta *audioCheckpointMeta,
	ckpt *workerentity.Checkpoint,
) (string, *workerentity.Checkpoint, bool, error) {
	if md := g.restore(ctx, req.ArtifactId, ckpt); md != "" {
		slog.InfoContext(ctx, "video storyboard restored from checkpoint",
			slog.String("artifact_id", req.ArtifactId.String()))
		return md, ckpt, true, nil
	}

	md, err := g.generate(ctx, req, payload, script, audioMeta)
	if err != nil {
		return "", ckpt, false, err
	}

	ckpt, err = g.save(ctx, req.ArtifactId, ckpt, md)
	if err != nil {
		return "", nil, false, errors.WithMessagef(err, "save video storyboard checkpoint failed")
	}

	return md, ckpt, false, nil
}

func (g *storyboardGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	script *videoScript,
	audioMeta *audioCheckpointMeta,
) (string, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioVideoOverviewStoryboardScene)

	// 开跑前清掉上一轮残留，避免读到陈旧分镜。
	if err := removeStoryboardFile(req.NotebookId, req.ArtifactId); err != nil {
		slog.WarnContext(ctx, "clear previous video storyboard draft failed",
			slog.String("artifact_id", req.ArtifactId.String()), slog.Any("err", err))
	}

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	segmentsMD := script.renderSegmentsMarkdown(audioMeta)

	msgs, err := RenderVideoStoryboard(
		ctx,
		sourceIds,
		payload.GetLanguage(),
		payload.GetTip(),
		segmentsMD,
	)
	if err != nil {
		return "", errors.WithMessagef(err, "render video storyboard prompt failed")
	}

	ag, err := newStoryboardAgent(g.deps, req)
	if err != nil {
		return "", err
	}

	storyTools, err := getStoryboardFileTools(req.NotebookId, req.ArtifactId)
	if err != nil {
		return "", err
	}
	if err := ag.AppendTools(storyTools); err != nil {
		return "", errors.Wrap(err, "bind storyboard tools failed")
	}

	if _, err := ag.React(ctx, msgs); err != nil {
		return "", errors.WithMessagef(err, "generate video storyboard react failed")
	}
	slog.InfoContext(ctx, fmt.Sprintf("generate video storyboard agent usage: %+v", ag.TokenUsage()))

	return readStoryboardDraft(ctx, req)
}

// readStoryboardDraft 读取草稿文件，只做空稿兜底，不解析也不校验 Markdown 结构。
func readStoryboardDraft(ctx context.Context, req *types.Request) (string, error) {
	raw, err := readStoryboardFile(req.NotebookId, req.ArtifactId)
	if err != nil {
		return "", err
	}

	md := normalizeStoryboardMarkdown(raw)
	if md == "" {
		return "", errors.Errorf("video storyboard draft is empty after agent run, artifact_id=%s", req.ArtifactId)
	}

	slog.InfoContext(ctx, "video storyboard draft ready",
		slog.String("artifact_id", req.ArtifactId.String()),
		slog.Int("chars", len(md)),
	)
	return md, nil
}

func (g *storyboardGenerator) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	markdown string,
) (*workerentity.Checkpoint, error) {
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField3(pkgstring.AsBytes(markdown))
	if err := g.checkpoints.Save(ctx, ckpt); err != nil {
		return nil, err
	}
	return ckpt, nil
}

func (g *storyboardGenerator) parse(_ context.Context, content string) (string, error) {
	md := normalizeStoryboardMarkdown(content)
	if md == "" {
		return "", fmt.Errorf("empty storyboard markdown")
	}
	return md, nil
}

func (g *storyboardGenerator) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) string {
	if ckpt == nil || len(ckpt.Field3) == 0 {
		return ""
	}
	md := strings.TrimSpace(string(ckpt.Field3))
	if md == "" {
		return ""
	}
	if _, err := g.parse(ctx, md); err != nil {
		slog.WarnContext(ctx, "video storyboard checkpoint invalid, treat as miss",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return ""
	}
	return md
}

// discardStale 上游口播或音频重算后清空 field3。
func (g *storyboardGenerator) discardStale(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) {
	if ckpt == nil || len(ckpt.Field3) == 0 {
		return
	}
	slog.WarnContext(ctx, "discarding stale video storyboard",
		slog.String("artifact_id", artifactId.String()),
		slog.Int("bytes", len(ckpt.Field3)),
	)
	ckpt.UpdateField3(nil)
	if err := g.checkpoints.Save(ctx, ckpt); err != nil {
		slog.ErrorContext(ctx, "clear stale video storyboard checkpoint failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
	}
}

func normalizeStoryboardMarkdown(content string) string {
	s := strings.TrimSpace(content)
	if s == "" {
		return ""
	}
	if after, ok := strings.CutPrefix(s, "```"); ok {
		s = strings.TrimSpace(after)
		if strings.HasPrefix(strings.ToLower(s), "markdown") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
			} else {
				s = ""
			}
		} else if strings.HasPrefix(strings.ToLower(s), "md") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
			} else {
				s = ""
			}
		}
		if idx := strings.LastIndex(s, "```"); idx >= 0 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
