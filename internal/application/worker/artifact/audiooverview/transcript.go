package audiooverview

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bytedance/sonic"
	einoschema "github.com/cloudwego/eino/schema"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio/voices"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	pkgjson "github.com/gonotelm-lab/gonotelm/pkg/encoding/json"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
)

type podcastTranscriptTurn struct {
	Speaker          string `json:"speaker"`
	Text             string `json:"text"`
	VoiceInstruction string `json:"voice_instruction"`
}

type podcastTranscriptSegment struct {
	Name     string                  `json:"name"`
	Dialogue []podcastTranscriptTurn `json:"dialogue"`
}

type podcastTranscriptExpectation struct {
	Title    string                     `json:"title"`
	Segments []podcastTranscriptSegment `json:"segments"`
}

// transcriptGenerator 基于大纲生成/恢复播客文字稿，写入 checkpoint.field2。
type transcriptGenerator struct {
	deps          *types.WorkerDeps
	checkpoints   *types.CheckpointStore
	audioProvider text2audio.Text2AudioProvider
}

func newTranscriptGenerator(
	deps *types.WorkerDeps,
	checkpoints *types.CheckpointStore,
	audioProvider text2audio.Text2AudioProvider,
) *transcriptGenerator {
	return &transcriptGenerator{deps: deps, checkpoints: checkpoints, audioProvider: audioProvider}
}

func (g *transcriptGenerator) ensure(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	ckpt *workerentity.Checkpoint,
	outline *podcastOutlineExpectation,
) (*podcastTranscriptExpectation, *workerentity.Checkpoint, bool, error) {
	if transcript := g.restore(ctx, req.ArtifactId, ckpt); transcript != nil {
		return transcript, ckpt, true, nil
	}

	transcript, err := g.generate(ctx, req, payload, outline)
	if err != nil {
		return nil, ckpt, false, err
	}

	ckpt, err = g.save(ctx, req.ArtifactId, ckpt, transcript)
	if err != nil {
		return nil, nil, false, errors.WithMessagef(err, "save transcript checkpoint failed")
	}

	return transcript, ckpt, false, nil
}

func (g *transcriptGenerator) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	outline *podcastOutlineExpectation,
) (*podcastTranscriptExpectation, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioAudioOverviewTranscriptScene)

	sourceIds := types.SourceIDsToStrings(req.SourceIds)
	msgs, err := RenderPodcastTranscript(ctx, sourceIds, payload.Language, payload.Tip, payload.Style, outline)
	if err != nil {
		return nil, errors.WithMessagef(err, "render podcast transcript prompt failed")
	}

	if skills := voices.GetProviderSkill(g.audioProvider); len(skills) > 0 {
		msgs = append(msgs, einoschema.UserMessage(skills))
	}

	ag, err := newAudioAgent(g.deps, req)
	if err != nil {
		return nil, err
	}

	step := types.NewAgentStepBuilder[*podcastTranscriptExpectation](ag, "podcast transcript").
		WithParse(func(ctx context.Context, content string) (*podcastTranscriptExpectation, error) {
			return g.parse(ctx, content, outline)
		}).
		WithRules(g.compensateRules).
		Build()
	return step.Run(ctx, msgs)
}

func (g *transcriptGenerator) compensateRules(error) []string {
	return []string{
		"JSON must contain only `title` and `segments`",
		"`title` must match the outline title",
		"`segments` count must match the outline; each element has `name` and `dialogue`",
		"`dialogue` is an array; each element has `speaker`, `text`, and `voice_instruction`",
		"`voice_instruction` is a voice direction",
	}
}

func (g *transcriptGenerator) save(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	transcript *podcastTranscriptExpectation,
) (*workerentity.Checkpoint, error) {
	data, err := sonic.Marshal(transcript)
	if err != nil {
		return nil, err
	}
	if ckpt == nil {
		ckpt = workerentity.NewCheckpoint(artifactId)
	}
	ckpt.UpdateField2(data)
	if err := g.checkpoints.Save(ctx, ckpt); err != nil {
		return nil, err
	}
	return ckpt, nil
}

func (g *transcriptGenerator) restore(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) *podcastTranscriptExpectation {
	if ckpt == nil || ckpt.Field2 == nil {
		return nil
	}
	var transcript podcastTranscriptExpectation
	if err := sonic.Unmarshal(ckpt.Field2, &transcript); err != nil {
		slog.WarnContext(ctx, "unmarshal transcript failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
		return nil
	}

	return &transcript
}

func (g *transcriptGenerator) parse(
	ctx context.Context,
	content string,
	outline *podcastOutlineExpectation,
) (*podcastTranscriptExpectation, error) {
	content = pkgstring.StripJSONPrefix(content)
	if content == "" {
		return nil, fmt.Errorf("empty output")
	}

	var expect podcastTranscriptExpectation
	decoder := pkgjson.Decoder{
		DisallowUnknownFields: true,
		LogOnDirectFailure: func(err error, _ []byte) {
			slog.DebugContext(ctx,
				"podcast transcript direct unmarshal did not match, fallback to json extraction",
				slog.Any("err", err),
				slog.String("raw_content", types.TruncateForLog(content)),
			)
		},
	}
	if err := decoder.Unmarshal(pkgstring.AsBytes(content), &expect); err != nil {
		slog.WarnContext(ctx, "podcast transcript output unmarshal failed after compatibility fallback",
			slog.Any("err", err),
			slog.String("raw_content", types.TruncateForLog(content)))
		return nil, err
	}

	expect.Title = strings.TrimSpace(expect.Title)

	if expect.Title == "" {
		return nil, fmt.Errorf("podcast transcript title is empty")
	}
	if len(expect.Segments) == 0 {
		return nil, fmt.Errorf("podcast transcript segments is empty")
	}
	if len(expect.Segments) != len(outline.Segments) {
		return nil, fmt.Errorf("transcript segments count %d != outline segments count %d",
			len(expect.Segments), len(outline.Segments))
	}

	for i := range expect.Segments {
		expect.Segments[i].Name = strings.TrimSpace(expect.Segments[i].Name)
		if expect.Segments[i].Name == "" {
			return nil, fmt.Errorf("transcript segment[%d] name is empty", i)
		}
		if len(expect.Segments[i].Dialogue) == 0 {
			return nil, fmt.Errorf("transcript segment[%d] dialogue is empty", i)
		}
		for j := range expect.Segments[i].Dialogue {
			turn := &expect.Segments[i].Dialogue[j]
			turn.Speaker = strings.TrimSpace(turn.Speaker)
			turn.Text = strings.TrimSpace(turn.Text)
			turn.VoiceInstruction = strings.TrimSpace(turn.VoiceInstruction)
			if turn.Speaker == "" {
				return nil, fmt.Errorf("segment[%d] dialogue[%d] speaker is empty", i, j)
			}
			if turn.Text == "" {
				return nil, fmt.Errorf("segment[%d] dialogue[%d] text is empty", i, j)
			}
			if turn.VoiceInstruction == "" {
				return nil, fmt.Errorf("segment[%d] dialogue[%d] voice_instruction is empty", i, j)
			}
		}
	}

	return &expect, nil
}
