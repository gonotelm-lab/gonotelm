package videooverview

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	audios "github.com/gonotelm-lab/multimodal/audio"
	"github.com/gonotelm-lab/multimodal/audio/schema"
	audioutil "github.com/gonotelm-lab/multimodal/audio/util"
	"golang.org/x/sync/errgroup"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/text2audio"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/storage"
	pkgaudio "github.com/gonotelm-lab/gonotelm/pkg/audio/wav"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
	"github.com/gonotelm-lab/gonotelm/pkg/safe"
)

// audioCheckpointVersion 音频产物缓存版本；朗读规则变化时递增，使旧音频失效并重新合成。
const audioCheckpointVersion = 2

// audioCheckpointMeta 持久化到 checkpoint.field2
type audioCheckpointMeta struct {
	Version int             `json:"version"`
	Parts   []audioLinePart `json:"parts"` // 逐句音频
}

func (m *audioCheckpointMeta) sortedAudioParts() []audioLinePart {
	if m == nil || len(m.Parts) == 0 {
		return nil
	}
	parts := append([]audioLinePart(nil), m.Parts...)
	sort.SliceStable(parts, func(i, j int) bool {
		if parts[i].SegmentIndex != parts[j].SegmentIndex {
			return parts[i].SegmentIndex < parts[j].SegmentIndex
		}
		if parts[i].LineIndex != parts[j].LineIndex {
			return parts[i].LineIndex < parts[j].LineIndex
		}
		return parts[i].Index < parts[j].Index
	})
	return parts
}

type audioLinePart struct {
	Index        int    `json:"index"`
	SegmentIndex int    `json:"segment_index"`
	LineIndex    int    `json:"line_index"`
	Text         string `json:"text"`
	StoreKey     string `json:"store_key"`
	DurationMs   int64  `json:"duration_ms"`
}

type synthesizedLine struct {
	SegmentIndex     int
	LineIndex        int
	Text             string
	VoiceInstruction string
}

// audioSynthizer 按口播稿逐句 TTS 并上传 OSS，写入 field2；不做整轨拼接。
type audioSynthizer struct {
	text2audio     *text2audio.Text2AudioGateway
	storage        storage.Storage
	checkpoints    *types.CheckpointStore
	downloadClient *http.Client

	provider    text2audio.Text2AudioProvider
	model       string
	concurrency int
}

func newAudioSynthizer(deps *types.WorkerDeps, checkpoints *types.CheckpointStore) *audioSynthizer {
	cfg := conf.WorkerGlobal().Studio.VideoOverview
	concurrency := cfg.AudioSynthConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	return &audioSynthizer{
		text2audio:     deps.Text2Audio,
		storage:        deps.ObjectStorage,
		checkpoints:    checkpoints,
		downloadClient: httpclient.NewBuilder(nil).WithTimeout(5 * time.Minute).Build(),
		provider:       cfg.AudioModelProvider,
		model:          cfg.AudioModel,
		concurrency:    concurrency,
	}
}

func (s *audioSynthizer) collectLines(script *videoScript) []synthesizedLine {
	if script == nil {
		return nil
	}
	var lines []synthesizedLine
	for si := range script.Segments {
		seg := &script.Segments[si]
		for li := range seg.Lines {
			line := &seg.Lines[li]
			lines = append(lines, synthesizedLine{
				SegmentIndex:     si,
				LineIndex:        li,
				Text:             line.Text,
				VoiceInstruction: resolveVoiceInstruction(script.VoiceBaseline, line.VoiceInstruction),
			})
		}
	}
	return lines
}

// resolveVoiceInstruction 取该行的 TTS 语气指令：有行级覆盖就用覆盖，否则继承全片基线。
func resolveVoiceInstruction(baseline, line string) string {
	if instruction := strings.TrimSpace(line); instruction != "" {
		return instruction
	}
	return strings.TrimSpace(baseline)
}

// generate 逐句 TTS；第二个返回值表示 field2 在进入本步前已完整，无需新合成。
func (s *audioSynthizer) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	script *videoScript,
	ckpt *workerentity.Checkpoint,
) (*audioCheckpointMeta, bool, error) {
	ctx = pkgcontext.WithSceneType(ctx, pkgcontext.StudioVideoOverviewAudioScene)

	if s.provider == "" {
		return nil, false, errors.ErrInner.Msgf("audio model provider is empty")
	}

	audioGenerator, err := s.text2audio.GetProvider(s.provider)
	if err != nil {
		return nil, false, errors.WithMessagef(err, "get text2audio provider failed")
	}

	voice, err := s.resolveNarratorVoice(payload.GetLanguage())
	if err != nil {
		return nil, false, err
	}

	lines := s.collectLines(script)
	if len(lines) == 0 {
		return nil, false, errors.ErrInner.Msgf("script has no lines to synthesize")
	}

	meta := s.restoreAudioMeta(ckpt)
	fullyCached := len(meta.Parts) >= len(lines)
	job := &lineSynthJob{
		payload:    payload,
		lines:      lines,
		meta:       meta,
		artifactId: req.ArtifactId,
		ckpt:       ckpt,
		generator:  audioGenerator,
		voice:      voice,
	}
	if err = s.synthesizePendingLines(ctx, job); err != nil {
		return nil, false, errors.WithMessagef(err, "synthesize pending lines failed")
	}

	if len(meta.Parts) < len(lines) {
		return nil, false, errors.ErrInner.Msgf(
			"video audio incomplete: have %d parts, want %d",
			len(meta.Parts), len(lines),
		)
	}

	slog.InfoContext(ctx, "video overview line audio synthesized",
		slog.String("artifact_id", req.ArtifactId.String()),
		slog.Int("lines", len(lines)),
		slog.Int("parts", len(meta.Parts)),
		slog.Bool("restored", fullyCached),
	)

	return meta, fullyCached, nil
}

type lineSynthResult struct {
	index        int
	segmentIndex int
	lineIndex    int
	text         string
	partKey      string
	pcm          *pkgaudio.PCM
}

type lineSynthJob struct {
	payload    *artifactentity.VideoOverviewPayload
	lines      []synthesizedLine
	meta       *audioCheckpointMeta
	artifactId valobj.Id
	ckpt       *workerentity.Checkpoint
	generator  audios.Generator
	voice      string
	callOpts   []audios.Option
}

func (s *audioSynthizer) synthesizePendingLines(ctx context.Context, job *lineSynthJob) error {
	done := len(job.meta.Parts)
	if done >= len(job.lines) {
		slog.DebugContext(ctx, "[video] all lines already synthesized, skip",
			slog.Int("total", len(job.lines)),
			slog.Int("done", done),
		)
		return nil
	}

	doneSet := make(map[int]bool, len(job.meta.Parts))
	for _, p := range job.meta.Parts {
		doneSet[p.Index] = true
	}

	pendingIdx := make([]int, 0, len(job.lines)-done)
	for i := range job.lines {
		if !doneSet[i] {
			pendingIdx = append(pendingIdx, i)
		}
	}
	if len(pendingIdx) == 0 {
		return nil
	}

	if opt := text2audio.WAVOption(s.provider); opt != nil {
		job.callOpts = append(job.callOpts, opt)
	}

	results := make(chan lineSynthResult, s.concurrency)

	var saveErr error
	var saveWG sync.WaitGroup
	safe.Go2(ctx, "video.synthesize.save_checkpoint", &saveWG, func(context.Context) {
		saveErr = s.persistLineSynthResults(ctx, job, results)
	})

	synthErr := s.synthesizeLinesConcurrent(ctx, job, pendingIdx, results)
	close(results)
	saveWG.Wait()

	if saveErr != nil {
		return saveErr
	}
	return synthErr
}

func (s *audioSynthizer) synthesizeLinesConcurrent(
	ctx context.Context,
	job *lineSynthJob,
	pendingIdx []int,
	results chan<- lineSynthResult,
) error {
	gp, gctx := errgroup.WithContext(ctx)
	gp.SetLimit(s.concurrency)

	for _, idx := range pendingIdx {
		i := idx
		gp.Go(func() error {
			return s.synthesizeOneLine(gctx, job, i, results)
		})
	}
	return gp.Wait()
}

func (s *audioSynthizer) synthesizeOneLine(
	ctx context.Context,
	job *lineSynthJob,
	index int,
	results chan<- lineSynthResult,
) error {
	line := &job.lines[index]

	ttsReq := &schema.Request{
		Model:       s.model,
		Text:        line.Text,
		Voice:       job.voice,
		Language:    text2audio.ResolveLineLanguage(line.Text, text2audio.AudioLang(string(job.payload.GetLanguage()))),
		Instruction: line.VoiceInstruction,
	}

	resp, err := job.generator.Generate(ctx, ttsReq, job.callOpts...)
	if err != nil {
		return errors.Wrapf(err, "tts generate failed for line %d (segment=%d line=%d)",
			index, line.SegmentIndex, line.LineIndex)
	}

	reader, err := audioutil.ResolveResponse(resp,
		audioutil.WithResolveContext(ctx),
		audioutil.WithResolveHttpClient(s.downloadClient),
	)
	if err != nil {
		return errors.WithMessagef(err, "resolve tts audio for line %d failed", index)
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "read tts audio for line %d failed, err=%v", index, err)
	}

	pcm, err := pkgaudio.Parse(raw)
	if err != nil {
		return errors.Wrapf(errors.ErrInner, "parse wav for line %d failed, err=%v", index, err)
	}

	partKey := s.formatLineAudioStoreKey(job.payload.NotebookId, job.artifactId, line.SegmentIndex, line.LineIndex)
	if err = s.storage.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         partKey,
		Body:        raw,
		ContentType: "audio/wav",
	}); err != nil {
		return errors.Wrapf(errors.ErrInner, "upload line audio for line %d failed, err=%v", index, err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case results <- lineSynthResult{
		index:        index,
		segmentIndex: line.SegmentIndex,
		lineIndex:    line.LineIndex,
		text:         line.Text,
		partKey:      partKey,
		pcm:          pcm,
	}:
		return nil
	}
}

func (s *audioSynthizer) persistLineSynthResults(
	ctx context.Context,
	job *lineSynthJob,
	results <-chan lineSynthResult,
) error {
	for r := range results {
		job.meta.Parts = append(job.meta.Parts, audioLinePart{
			Index:        r.index,
			SegmentIndex: r.segmentIndex,
			LineIndex:    r.lineIndex,
			Text:         r.text,
			StoreKey:     r.partKey,
			DurationMs:   r.pcm.DurationMs(),
		})
		snap, snapErr := s.snapshotAudioCheckpoint(job.ckpt, job.meta)
		if snapErr != nil {
			slog.WarnContext(ctx, "snapshot video audio checkpoint failed",
				slog.String("artifact_id", job.artifactId.String()),
				slog.Any("err", snapErr))
		} else if err := s.checkpoints.Save(ctx, snap); err != nil {
			slog.WarnContext(ctx, "persist video audio checkpoint failed",
				slog.String("artifact_id", job.artifactId.String()),
				slog.Any("err", err))
		}

		slog.DebugContext(ctx, "video line audio synthesized",
			slog.String("artifact_id", job.artifactId.String()),
			slog.Int("line", r.index),
			slog.Int("segment", r.segmentIndex),
			slog.Int64("duration_ms", r.pcm.DurationMs()),
		)
	}
	return nil
}

// discardStale 口播稿重算后清理 field2 中的逐句音频。
func (s *audioSynthizer) discardStale(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) {
	meta := s.restoreAudioMeta(ckpt)
	if meta == nil || len(meta.Parts) == 0 {
		if ckpt != nil && len(ckpt.Field2) > 0 {
			ckpt.UpdateField2(nil)
			_ = s.checkpoints.Save(ctx, ckpt)
		}
		return
	}

	keys := make([]string, 0, len(meta.Parts))
	for _, p := range meta.Parts {
		keys = append(keys, p.StoreKey)
	}
	slog.WarnContext(ctx, "discarding stale video line audio",
		slog.String("artifact_id", artifactId.String()),
		slog.Int("part_count", len(keys)),
	)
	if err := s.storage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{Keys: keys}); err != nil {
		slog.ErrorContext(ctx, "cleanup stale video audio failed",
			slog.Int("count", len(keys)), slog.Any("err", err))
	}

	ckpt.UpdateField2(nil)
	if err := s.checkpoints.Save(ctx, ckpt); err != nil {
		slog.ErrorContext(ctx, "clear stale video audio checkpoint failed",
			slog.String("artifact_id", artifactId.String()), slog.Any("err", err))
	}
}

func (s *audioSynthizer) snapshotAudioCheckpoint(ckpt *workerentity.Checkpoint, meta *audioCheckpointMeta) (*workerentity.Checkpoint, error) {
	if ckpt == nil {
		return nil, errors.New("video audio checkpoint is nil")
	}
	data, err := sonic.Marshal(meta)
	if err != nil {
		return nil, err
	}
	ckpt.UpdateField2(data)
	snap := *ckpt
	return &snap, nil
}

func (s *audioSynthizer) restoreAudioMeta(ckpt *workerentity.Checkpoint) *audioCheckpointMeta {
	if ckpt == nil || len(ckpt.Field2) == 0 {
		return &audioCheckpointMeta{Version: audioCheckpointVersion}
	}
	var meta audioCheckpointMeta
	if err := sonic.Unmarshal(ckpt.Field2, &meta); err != nil {
		return &audioCheckpointMeta{Version: audioCheckpointVersion}
	}
	if meta.Version != audioCheckpointVersion {
		slog.Warn("video audio checkpoint version mismatch, regenerate all lines",
			slog.Int("cached_version", meta.Version),
			slog.Int("current_version", audioCheckpointVersion))
		return &audioCheckpointMeta{Version: audioCheckpointVersion}
	}
	return &meta
}

func (s *audioSynthizer) formatLineAudioStoreKey(
	notebookId, artifactId valobj.Id,
	segmentIndex, lineIndex int,
) string {
	return fmt.Sprintf("tmp/artifact/%s/%s/video/audio_%d-%d.wav",
		notebookId.String(), artifactId.String(), segmentIndex, lineIndex)
}
