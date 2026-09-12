package videooverview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	einoschema "github.com/cloudwego/eino/schema"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const storyboardCompensate = 3

// storyboardGenerator 基于大纲 + 带时长口播轨生成分镜 Markdown，写入 checkpoint.field3。
// 模型通过内存工具 AppendStoryBoardShot / EditStoryBoardShot / ReadStoryBoardShot 增量写入，避免一次吐完整稿。
type storyboardGenerator struct {
	deps        *types.WorkerDeps
	checkpoints *checkpointStore
}

func newStoryboardGenerator(deps *types.WorkerDeps, checkpoints *checkpointStore) *storyboardGenerator {
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
	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	outlineMD := script.renderOutlineMarkdown()
	narrationMD := script.renderNarrationMarkdown(audioMeta)

	msgs, err := RenderVideoStoryboard(
		ctx,
		sourceIds,
		payload.GetLanguage(),
		payload.GetTip(),
		payload.GetVisualStyle(),
		outlineMD,
		narrationMD,
	)
	if err != nil {
		return "", errors.WithMessagef(err, "render video storyboard prompt failed")
	}

	ag, err := newStoryboardAgent(g.deps, req)
	if err != nil {
		return "", err
	}

	doc := newStoryboardDoc()
	expected := expectedAudioIDsFromMeta(audioMeta)
	doc.SetExpectedAudioIDs(expected)

	storyTools, err := doc.getBindedTools()
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

	if err := g.ensureStoryboardCoverage(ctx, ag, doc, audioMeta); err != nil {
		return "", err
	}

	md, err := g.parse(ctx, doc.Markdown())
	if err != nil {
		return "", errors.WithMessagef(err,
			"video storyboard draft empty after agent run (shots=%d)", doc.ShotCount())
	}
	return md, nil
}

// ensureStoryboardCoverage 校验口播全覆盖与 shot index 连续；缺失则补偿续写。
func (g *storyboardGenerator) ensureStoryboardCoverage(
	ctx context.Context,
	ag *types.Agent,
	doc *storyboardDoc,
	audioMeta *audioCheckpointMeta,
) error {
	for attempt := 0; attempt <= storyboardCompensate; attempt++ {
		missingAudio, indexGaps := doc.coverageReport()
		if len(missingAudio) == 0 && len(indexGaps) == 0 {
			return nil
		}
		if attempt == storyboardCompensate {
			return fmt.Errorf(
				"storyboard incomplete after %d compensate attempts: missing_audio_ids=%v shot_index_gaps=%v shots=%d",
				storyboardCompensate, missingAudio, indexGaps, doc.ShotCount(),
			)
		}

		slog.WarnContext(ctx, "video storyboard incomplete, compensating",
			slog.Int("attempt", attempt+1),
			slog.Int("missing_audio", len(missingAudio)),
			slog.Int("index_gaps", len(indexGaps)),
			slog.Int("shots", doc.ShotCount()),
			slog.Any("missing_audio_ids_sample", truncateStrings(missingAudio, 12)),
			slog.Any("shot_index_gaps_sample", truncateInts(indexGaps, 12)),
		)

		fixMsg := buildStoryboardCompensateMessage(missingAudio, indexGaps, audioMeta)
		if _, err := ag.React(ctx, []*einoschema.Message{fixMsg}); err != nil {
			return errors.WithMessagef(err, "compensate storyboard failed (attempt %d)", attempt+1)
		}
	}
	return nil
}

func buildStoryboardCompensateMessage(
	missingAudio []string,
	indexGaps []int,
	audioMeta *audioCheckpointMeta,
) *einoschema.Message {
	var b strings.Builder
	b.WriteString("Storyboard draft is INCOMPLETE. Do NOT restart from scratch.\n")
	b.WriteString("Keep existing shots. Fill ONLY the gaps with AppendStoryBoardShot.\n\n")

	if len(indexGaps) > 0 {
		fmt.Fprintf(&b, "Missing shot indexes (must become contiguous 1..N): %v\n", indexGaps)
		b.WriteString("Use these indexes when appending the missing shots.\n\n")
	}
	if len(missingAudio) > 0 {
		fmt.Fprintf(&b, "Missing audio_ids (%d): must each appear in exactly one shot.\n", len(missingAudio))
		textByID := audioTextByID(audioMeta)
		for _, id := range missingAudio {
			text := textByID[id]
			if text == "" {
				fmt.Fprintf(&b, "- %s\n", id)
			} else {
				fmt.Fprintf(&b, "- %s — %s\n", id, text)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("Rules:\n")
	b.WriteString("1. Append at most 10 shots per tool call; continue until coverage is complete.\n")
	b.WriteString("2. Shot indexes must be contiguous 1..N; fill every gap index.\n")
	b.WriteString("3. One shot MAY cover multiple consecutive audio_ids when they share the same visual logic.\n")
	b.WriteString("4. Every narration audio_id must appear; do not omit any clip.\n")
	b.WriteString("5. Do NOT add section/板块 headings; only ## Shot blocks.\n")
	b.WriteString("6. After filling, reply with one short confirmation only.\n")

	return einoschema.UserMessage(b.String())
}

func audioTextByID(meta *audioCheckpointMeta) map[string]string {
	out := make(map[string]string)
	if meta == nil {
		return out
	}
	for _, p := range meta.sortedAudioParts() {
		id := fmt.Sprintf("audio_%d-%d", p.SegmentIndex, p.LineIndex)
		out[id] = strings.TrimSpace(p.Text)
	}
	return out
}

func truncateStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func truncateInts(in []int, n int) []int {
	if len(in) <= n {
		return in
	}
	return in[:n]
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
	if err := g.checkpoints.save(ctx, ckpt); err != nil {
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
	if err := g.checkpoints.save(ctx, ckpt); err != nil {
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

// formatTimecode 将毫秒转为 MM:SS.mmm
func formatTimecode(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	totalMs := ms
	minutes := totalMs / 60_000
	rem := totalMs % 60_000
	seconds := rem / 1000
	millis := rem % 1000
	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, millis)
}
