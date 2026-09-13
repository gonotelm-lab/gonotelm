package videooverview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio/voices"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const scriptCompensate = 3

// videoScriptLine 逐句口播：text 送 TTS 朗读，voice_instruction 控制该句语气/节奏。
type videoScriptLine struct {
	Text             string `json:"text"`
	VoiceInstruction string `json:"voice_instruction"`
}

// videoScript 口播稿：板块构思（content）+ 逐句旁白（lines），写入 checkpoint.field1。
type videoScriptSegment struct {
	Name    string            `json:"name"`
	Content string            `json:"content"`
	Lines   []videoScriptLine `json:"lines"`
}

type videoScript struct {
	Title    string               `json:"title"`
	Segments []videoScriptSegment `json:"segments"`
}

func (s *videoScript) renderOutlineMarkdown() string {
	if s == nil {
		return "(no outline)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimSpace(s.Title))
	for i, seg := range s.Segments {
		fmt.Fprintf(&b, "## Segment %d · %s\n\n", i, strings.TrimSpace(seg.Name))
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(seg.Content))
	}
	return strings.TrimSpace(b.String())
}

func (s *videoScript) renderNarrationMarkdown(meta *audioCheckpointMeta) string {
	parts := meta.sortedAudioParts()
	if len(parts) == 0 {
		return "(no narration audio track)"
	}

	segName := func(si int) string {
		if s != nil && si >= 0 && si < len(s.Segments) {
			return s.Segments[si].Name
		}
		return ""
	}

	var b strings.Builder
	b.WriteString("Sorted by `segment_index` → `line_index`; timecodes are cumulative start–end.\n\n")

	var cursor int64
	curSeg := -1
	for _, p := range parts {
		if p.SegmentIndex != curSeg {
			curSeg = p.SegmentIndex
			name := segName(curSeg)
			if name != "" {
				fmt.Fprintf(&b, "### Segment %d · %s\n\n", curSeg, name)
			} else {
				fmt.Fprintf(&b, "### Segment %d\n\n", curSeg)
			}
		}
		start := cursor
		end := cursor + p.DurationMs
		cursor = end
		fmt.Fprintf(&b, "- **audio_id**: `audio_%d-%d`\n", p.SegmentIndex, p.LineIndex)
		fmt.Fprintf(&b, "  - timecode: `%s – %s`\n", formatTimecode(start), formatTimecode(end))
		fmt.Fprintf(&b, "  - duration_ms: %d\n", p.DurationMs)
		fmt.Fprintf(&b, "  - text: %s\n\n", strings.TrimSpace(p.Text))
	}
	fmt.Fprintf(&b, "Total about `%s` (%d ms).\n", formatTimecode(cursor), cursor)
	fmt.Fprintf(&b, "\n## Coverage requirement\n\n")
	fmt.Fprintf(&b, "There are **%d** spoken clips above. Every `audio_id` MUST appear in some shot — none may be omitted.\n", len(parts))
	fmt.Fprintf(&b, "One shot may cover multiple **consecutive** clips when they serve the same on-screen logic.\n")
	fmt.Fprintf(&b, "Shot indexes must be contiguous from 1; do not emit section/板块 headings in the storyboard.\n")
	fmt.Fprintf(&b, "Do not cut shots by stopwatch limits; cut when the visual argument changes.\n")
	return strings.TrimSpace(b.String())
}

// scriptGenerator 一次探索生成口播稿（先构思板块再写 lines），写入 checkpoint.field1。
type scriptGenerator struct {
	deps          *types.WorkerDeps
	checkpoints   *checkpointStore
	audioProvider text2audio.Text2AudioProvider
}

func newScriptGenerator(deps *types.WorkerDeps, checkpoints *checkpointStore) *scriptGenerator {
	return &scriptGenerator{
		deps:          deps,
		checkpoints:   checkpoints,
		audioProvider: conf.WorkerGlobal().Studio.VideoOverview.AudioModelProvider,
	}
}

func (g *scriptGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	ckpt *workerentity.Checkpoint,
) (*videoScript, *workerentity.Checkpoint, bool, error) {
	if script := g.restore(ctx, req.ArtifactId, ckpt); script != nil {
		slog.InfoContext(ctx, "video script restored from checkpoint",
			slog.String("artifact_id", req.ArtifactId.String()))
		return script, ckpt, true, nil
	}

	script, err := g.generate(ctx, req, payload)
	if err != nil {
		return nil, ckpt, false, err
	}

	ckpt, err = g.save(ctx, req.ArtifactId, ckpt, script)
	if err != nil {
		return nil, nil, false, errors.WithMessagef(err, "save video script checkpoint failed")
	}

	return script, ckpt, false, nil
}

func (g *scriptGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
) (*videoScript, error) {
	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderVideoScript(ctx, sourceIds, payload.GetLanguage(), payload.GetTip(), payload.GetVisualStyle())
	if err != nil {
		return nil, errors.WithMessagef(err, "render video script prompt failed")
	}

	if skill := voices.GetProviderSkill(g.audioProvider); skill != "" {
		msgs = append(msgs, einoschema.UserMessage(skill))
	}

	ag, err := newVideoScriptAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*videoScript](ag, "video script").
		WithParse(g.parse).
		WithRetry(scriptCompensate).
		WithRules(g.compensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func (g *scriptGenerator) compensateRules(validateErr error) []string {
	rules := []string{
		"JSON must contain only `title` and `segments`",
		"`segments` is a non-empty array; each element has `name`, `content`, and `lines`",
		"`content` is the segment plan (what to cover), not the spoken narration",
		"`lines` is an array of objects; each element has `text` and `voice_instruction`",
		"`text` is one short spoken sentence for TTS, no long paragraphs",
		"`text` must be spoken-style plain text, no markdown, emoji, urls, parentheses asides, or newlines",
		"`voice_instruction` is a natural-language voice direction for that line, not a restatement of `text`; keep ONE baseline across the whole script — spoken-narration style, medium pace, consistent tone/intonation — that fits the video's overall style and the source content's register, with only slight local adjustments (never fast here / slow there)",
		"resolve ambiguous readings in `text` by context (Roman numerals, single letters, English acronyms, mixed symbols) into unambiguous spoken Chinese/phonetic form for TTS; ",
		"keep the same reading consistent across the script",
		"never include system internals in title/name/content/text/voice_instruction: source ids, tool names, checkpoint/artifact fields, or meta narration about tools",
		"plan each segment (name+content) first, then write lines",
	}
	if validateErr != nil {
		rules = append(rules, "Error: "+validateErr.Error())
	}
	return rules
}

func (g *scriptGenerator) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	script *videoScript,
) (*workerentity.Checkpoint, error) {
	data, err := sonic.Marshal(script)
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

func (g *scriptGenerator) parse(ctx context.Context, content string) (*videoScript, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var script videoScript
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"video script direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &script); err != nil {
		slog.WarnContext(ctx, "video script output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	script.Title = strings.TrimSpace(script.Title)
	if script.Title == "" {
		return nil, fmt.Errorf("video script title is empty")
	}
	if len(script.Segments) == 0 {
		return nil, fmt.Errorf("video script segments is empty")
	}

	for i := range script.Segments {
		seg := &script.Segments[i]
		seg.Name = strings.TrimSpace(seg.Name)
		seg.Content = strings.TrimSpace(seg.Content)
		if seg.Name == "" {
			return nil, fmt.Errorf("segment[%d] name is empty", i)
		}
		if seg.Content == "" {
			return nil, fmt.Errorf("segment[%d] content is empty", i)
		}
		if len(seg.Lines) == 0 {
			return nil, fmt.Errorf("segment[%d] lines is empty", i)
		}

		for j := range seg.Lines {
			line := &seg.Lines[j]
			line.Text = strings.Join(strings.Fields(line.Text), " ")
			line.VoiceInstruction = strings.TrimSpace(line.VoiceInstruction)
			if line.Text == "" {
				return nil, fmt.Errorf("segment[%d] lines[%d] text is empty", i, j)
			}
		}
	}

	return &script, nil
}

func (g *scriptGenerator) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *videoScript {
	if ckpt == nil || ckpt.Field1 == nil {
		return nil
	}
	var script videoScript
	if err := sonic.Unmarshal(ckpt.Field1, &script); err != nil {
		slog.WarnContext(ctx, "unmarshal video script failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}
	if script.Title == "" || len(script.Segments) == 0 {
		return nil
	}
	for _, seg := range script.Segments {
		if len(seg.Lines) == 0 {
			slog.WarnContext(ctx, "video script checkpoint missing lines, treat as miss",
				slog.String("artifact_id", artifactId.String()))
			return nil
		}
	}
	return &script
}
