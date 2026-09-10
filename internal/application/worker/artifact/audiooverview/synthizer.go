package audiooverview

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/httpclient"
)

type AudioStorageResult struct {
	StoreKey    string                  `json:"store_key"`
	ContentType string                  `json:"content_type"`
	Audio       *AudioStorageResultMeta `json:"audio,omitempty"`
}

type AudioStorageResultMeta struct {
	Format        string `json:"format"`
	NumChannels   int    `json:"channels"`
	SampleRate    int    `json:"sample_rate"`
	BitsPerSample int    `json:"bits_per_sample"`
	Size          int    `json:"size"`
	DurationMs    int64  `json:"duration_ms"`
}

type synthesizedTurn struct {
	SpeakerName string
	Text        string
	Instruction string
}

// audioCheckpointMeta 持久化到 checkpoint.Field3，记录已成功合成并上传的逐段音频元信息，
// 用于跨进程断点重试。
type audioCheckpointMeta struct {
	NumChannels   uint16          `json:"num_channels"`
	SampleRate    uint32          `json:"sample_rate"`
	BitsPerSample uint16          `json:"bits_per_sample"`
	Parts         []audioTurnPart `json:"parts"`
}

type audioTurnPart struct {
	Index    int    `json:"index"`
	StoreKey string `json:"store_key"`
}

// audioSynthizer 负责播客音频合成：逐轮 TTS、上传 OSS、拼接成完整 WAV、断点续跑与陈旧音频清理。
type audioSynthizer struct {
	text2audio     *text2audio.Text2AudioGateway
	storage        storage.Storage
	checkpoints    *checkpointStore
	downloadClient *http.Client

	provider    text2audio.Text2AudioProvider
	model       string
	concurrency int
}

func newAudioSynthizer(deps *types.WorkerDeps, checkpoints *checkpointStore) *audioSynthizer {
	cfg := conf.WorkerGlobal().Studio.AudioOverview
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

// collectTurns 将 transcript 展平为按播放顺序排列的发言序列。
func collectTurns(t *podcastTranscriptExpectation) []synthesizedTurn {
	if t == nil {
		return nil
	}
	var turns []synthesizedTurn
	for si := range t.Segments {
		seg := &t.Segments[si]
		for ti := range seg.Dialogue {
			turn := &seg.Dialogue[ti]
			turns = append(turns, synthesizedTurn{
				SpeakerName: turn.Speaker,
				Text:        turn.Text,
				Instruction: turn.VoiceInstruction,
			})
		}
	}

	return turns
}

// buildSpeakerVoiceMap 取当前 style 对应 episode 的 speakers，按 speaker 名 → provider voice 建映射，
// 并按请求语言选择对应音色。
func buildSpeakerVoiceMap(
	style artifactentity.AudioOverviewStyle,
	provider text2audio.Text2AudioProvider,
	lang artifactentity.Language,
) (map[string]string, error) {
	ep, ok := artifactentity.BuiltinEpisodes[style]
	if !ok {
		ep, ok = artifactentity.BuiltinEpisodes[artifactentity.AudioOverviewStyleDefault()]
		if !ok {
			return nil, errors.ErrInner.Msgf("no builtin episode for style %q", style)
		}
	}

	providerKey := provider.String()
	m := make(map[string]string, len(ep.Speakers))
	for _, sp := range ep.Speakers {
		langMap, ok := sp.Voices[providerKey]
		if !ok {
			return nil, errors.ErrInner.Msgf(
				"speaker %q has no voice mapping for provider %q",
				sp.Name, providerKey,
			)
		}
		voice := resolveVoice(langMap, lang)
		if voice == "" {
			return nil, errors.ErrInner.Msgf(
				"speaker %q has no voice for language %q in provider %q",
				sp.Name, lang, providerKey,
			)
		}
		m[sp.Name] = voice
	}

	return m, nil
}

func resolveVoice(langMap map[string]string, lang artifactentity.Language) string {
	return langMap[string(lang)]
}

// Generate 逐段调用 TTS 并上传中间 WAV 到 OSS，最后下载/拼接、上传最终 WAV 并清理中间键。
// 重试时跳过已合成 index、稀疏补齐失败的 index。
func (s *audioSynthizer) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.AudioOverviewPayload,
	transcript *podcastTranscriptExpectation,
	ckpt *workerentity.Checkpoint,
) (*AudioStorageResult, error) {
	slog.DebugContext(ctx, "[audio] generateAudio start",
		slog.String("artifact_id", req.ArtifactId.String()),
		slog.String("notebook_id", payload.NotebookId.String()),
		slog.String("style", string(payload.Style)),
	)

	provider := s.provider
	if provider == "" {
		return nil, errors.ErrInner.Msgf("audio model provider is empty")
	}

	slog.DebugContext(ctx, "[audio] provider config",
		slog.String("provider", provider.String()),
		slog.String("model", s.model),
	)

	audioGenerator, err := s.text2audio.GetProvider(provider)
	if err != nil {
		return nil, errors.WithMessagef(err, "get text2audio provider failed")
	}

	voiceByID, err := buildSpeakerVoiceMap(payload.Style, provider, payload.Language)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "[audio] speaker-voice mapping",
		slog.Int("speaker_count", len(voiceByID)),
	)

	turns := collectTurns(transcript)
	if len(turns) == 0 {
		return nil, errors.ErrInner.Msgf("transcript has no turns to synthesize")
	}

	slog.DebugContext(ctx, "[audio] turns collected",
		slog.Int("total_turns", len(turns)),
	)

	meta := restoreAudioMeta(ckpt)

	slog.DebugContext(ctx, "[audio] checkpoint restored",
		slog.Int("done_parts", len(meta.Parts)),
		slog.Int("pending", len(turns)-len(meta.Parts)),
	)

	if err = s.synthesizePendingTurns(ctx, payload, turns, meta, req.ArtifactId, ckpt, audioGenerator, voiceByID); err != nil {
		return nil, errors.WithMessagef(err, "synthesize pending turns failed")
	}

	slog.DebugContext(ctx, "[audio] assembling ordered PCMs",
		slog.Int("total_turns", len(turns)),
	)

	orderedPCMs, err := s.assembleOrderedPCMs(ctx, len(turns), meta)
	if err != nil {
		slog.WarnContext(ctx, "assemble ordered pcms failed, resetting checkpoint and re-synthesizing",
			slog.String("artifact_id", req.ArtifactId.String()),
			slog.Any("err", err),
		)
		meta.Parts = nil
		meta.NumChannels = 0
		meta.SampleRate = 0
		meta.BitsPerSample = 0
		if err := s.persistAudioCheckpoint(ctx, req.ArtifactId, ckpt, meta); err != nil {
			slog.ErrorContext(ctx, "reset audio checkpoint failed",
				slog.String("artifact_id", req.ArtifactId.String()),
				slog.Any("err", err),
			)
		}
		if err = s.synthesizePendingTurns(ctx, payload, turns, meta, req.ArtifactId, ckpt, audioGenerator, voiceByID); err != nil {
			return nil, errors.WithMessagef(err, "re-synthesize pending turns failed")
		}
		orderedPCMs, err = s.assembleOrderedPCMs(ctx, len(turns), meta)
		if err != nil {
			return nil, errors.WithMessagef(err, "re-assemble ordered pcms failed")
		}
	}

	slog.DebugContext(ctx, "[audio] concatenating PCMs",
		slog.Int("pcm_count", len(orderedPCMs)),
	)

	wavBytes, merged, err := pkgaudio.ConcatPCMs(orderedPCMs)
	if err != nil {
		return nil, errors.WithMessagef(err, "concat podcast audio failed")
	}

	slog.DebugContext(ctx, "[audio] WAV concatenated",
		slog.Int("wav_size", len(wavBytes)),
		slog.Int("sample_rate", int(merged.SampleRate)),
		slog.Int("channels", int(merged.NumChannels)),
	)

	storeKey := formatAudioStoreKey(payload.NotebookId, req.ArtifactId)
	if err = s.storage.UploadObject(ctx, &storage.UploadObjectRequest{
		Key:         storeKey,
		Body:        wavBytes,
		ContentType: "audio/wav",
	}); err != nil {
		return nil, errors.WithMessagef(err, "upload podcast audio failed")
	}

	slog.InfoContext(ctx, "podcast audio synthesized and uploaded",
		slog.String("artifact_id", req.ArtifactId.String()),
		slog.String("store_key", storeKey),
		slog.Int("turns", len(turns)),
		slog.Int("size", len(wavBytes)),
	)

	// 全部成功后清理逐段中间音频
	s.cleanupIntermediateAudio(ctx, meta)

	return &AudioStorageResult{
		StoreKey:    storeKey,
		ContentType: "audio/wav",
		Audio: &AudioStorageResultMeta{
			Format:        "wav",
			NumChannels:   int(merged.NumChannels),
			SampleRate:    int(merged.SampleRate),
			BitsPerSample: int(merged.BitsPerSample),
			Size:          len(wavBytes),
			DurationMs:    merged.DurationMs(),
		},
	}, nil
}

// synthesizePendingTurns 并发执行尚未出现在 meta.Parts 中的 turn 的 TTS。
// 每段成功后立即把 WAV 上传至 OSS 并增量写 checkpoint.Field3；任一失败终止剩余任务。
func (s *audioSynthizer) synthesizePendingTurns(
	ctx context.Context,
	payload *artifactentity.AudioOverviewPayload,
	turns []synthesizedTurn,
	meta *audioCheckpointMeta,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	audioGenerator audios.Generator,
	voiceByID map[string]string,
) error {
	done := len(meta.Parts)
	if done >= len(turns) {
		slog.DebugContext(ctx, "[audio] all turns already synthesized, skip",
			slog.Int("total", len(turns)),
			slog.Int("done", done),
		)
		return nil
	}

	slog.DebugContext(ctx, "[audio] synthesizePendingTurns start",
		slog.Int("total", len(turns)),
		slog.Int("done", done),
		slog.Int("pending", len(turns)-done),
		slog.String("provider", s.provider.String()),
		slog.String("model", s.model),
		slog.String("language", string(payload.Language)),
	)

	gp, gctx := errgroup.WithContext(ctx)
	gp.SetLimit(s.concurrency)

	var callOpts []audios.Option
	if opt := text2audio.WAVOption(s.provider); opt != nil {
		callOpts = append(callOpts, opt)
	}

	// protected 内 meta + ckpt 的增量读写：每段成功立刻持久化 checkpoint。
	var mu sync.Mutex

	doneSet := make(map[int]bool, len(meta.Parts))
	for _, p := range meta.Parts {
		doneSet[p.Index] = true
	}

	for i := range turns {
		if doneSet[i] {
			continue
		}
		gp.Go(func() error {
			turn := &turns[i]
			voice, ok := voiceByID[turn.SpeakerName]
			if !ok {
				err := errors.ErrInner.Msgf(
					"no voice mapping for speaker %q (turn %d)",
					turn.SpeakerName, i,
				)
				slog.ErrorContext(gctx, "[audio] turn failed: no voice mapping",
					slog.Int("turn_index", i),
					slog.String("speaker", turn.SpeakerName),
					slog.Any("err", err),
				)
				return err
			}

			ttsReq := &schema.Request{
				Model:       s.model,
				Text:        turn.Text,
				Voice:       voice,
				Language:    text2audio.AudioLang(string(payload.Language)),
				Instruction: turn.Instruction,
			}

			resp, err := audioGenerator.Generate(gctx, ttsReq, callOpts...)
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn TTS generate failed",
					slog.Int("turn_index", i),
					slog.String("speaker", turn.SpeakerName),
					slog.String("voice", voice),
					slog.Any("err", err),
				)
				return errors.Wrapf(err, "tts generate failed for turn %d (speaker=%s)", i, turn.SpeakerName)
			}

			reader, err := audioutil.ResolveResponse(resp,
				audioutil.WithResolveContext(gctx),
				audioutil.WithResolveHttpClient(s.downloadClient),
			)
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn resolve response failed",
					slog.Int("turn_index", i),
					slog.Any("err", err),
				)
				return errors.WithMessagef(err, "resolve tts audio for turn %d failed", i)
			}
			defer reader.Close()

			raw, err := io.ReadAll(reader)
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn read audio failed",
					slog.Int("turn_index", i),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner, "read tts audio for turn %d failed, err=%v", i, err)
			}

			pcm, err := pkgaudio.Parse(raw)
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn parse wav failed",
					slog.Int("turn_index", i),
					slog.Int("audio_bytes", len(raw)),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner, "parse wav for turn %d failed, err=%v", i, err)
			}

			// meta 为并发共享对象，格式校验读取也需持锁
			mu.Lock()
			fmtErr := assertOrInitFormat(meta, pcm)
			mu.Unlock()
			if fmtErr != nil {
				slog.ErrorContext(gctx, "[audio] turn format incompatible",
					slog.Int("turn_index", i),
					slog.Any("err", fmtErr),
				)
				return errors.Wrapf(errors.ErrInner, "format incompatible for turn %d, err=%v", i, fmtErr)
			}

			partKey := formatIntermediateAudioStoreKey(payload.NotebookId, artifactId, i)
			if err = s.storage.UploadObject(gctx, &storage.UploadObjectRequest{
				Key:         partKey,
				Body:        raw,
				ContentType: "audio/wav",
			}); err != nil {
				slog.ErrorContext(gctx, "[audio] turn upload intermediate audio failed",
					slog.Int("turn_index", i),
					slog.String("part_key", partKey),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner, "upload intermediate audio for turn %d failed, err=%v", i, err)
			}

			// 落 checkpoint 必须在 OSS 上传成功之后；锁内只做快照，DB 写入在锁外。
			mu.Lock()
			if meta.NumChannels == 0 {
				meta.NumChannels = pcm.NumChannels
				meta.SampleRate = pcm.SampleRate
				meta.BitsPerSample = pcm.BitsPerSample
			}
			meta.Parts = append(meta.Parts, audioTurnPart{Index: i, StoreKey: partKey})
			snap, snapErr := snapshotAudioCheckpoint(ckpt, meta)
			mu.Unlock()

			if snapErr != nil {
				slog.WarnContext(gctx, "snapshot audio checkpoint failed",
					slog.String("artifact_id", artifactId.String()),
					slog.Any("err", snapErr))
			} else if saveErr := s.checkpoints.save(gctx, snap); saveErr != nil {
				slog.WarnContext(gctx, "persist audio checkpoint failed",
					slog.String("artifact_id", artifactId.String()),
					slog.Any("err", saveErr))
				// checkpoint 失败不阻断本轮，但下次重试将重复上传这段；可接受。
			}

			slog.DebugContext(gctx, "podcast audio turn synthesized",
				slog.String("artifact_id", artifactId.String()),
				slog.Int("turn", i),
				slog.Int("pcm_bytes", len(pcm.Data)),
			)
			return nil
		})
	}

	return gp.Wait()
}

// assembleOrderedPCMs 按 turn index 从 OSS 读回所有逐段 PCM，形成 [0..N-1] 保序切片。
func (s *audioSynthizer) assembleOrderedPCMs(
	ctx context.Context,
	total int,
	meta *audioCheckpointMeta,
) ([]*pkgaudio.PCM, error) {
	if len(meta.Parts) != total {
		return nil, errors.ErrInner.Msgf(
			"audio checkpoint mismatch: have %d parts, want %d",
			len(meta.Parts), total,
		)
	}

	partsByIndex := make(map[int]string, len(meta.Parts))
	for _, p := range meta.Parts {
		partsByIndex[p.Index] = p.StoreKey
	}

	out := make([]*pkgaudio.PCM, total)
	for i := range total {
		key, ok := partsByIndex[i]
		if !ok {
			return nil, errors.ErrInner.Msgf("missing intermediate audio for turn %d", i)
		}
		pcm, err := s.downloadTurnPCM(ctx, key)
		if err != nil {
			return nil, errors.WithMessagef(err, "download intermediate audio for turn %d failed", i)
		}
		out[i] = pcm
	}
	return out, nil
}

// downloadTurnPCM 从 OSS 下载逐段 WAV 并解析为 PCM。
func (s *audioSynthizer) downloadTurnPCM(ctx context.Context, key string) (*pkgaudio.PCM, error) {
	resp, err := s.storage.GetObject(ctx, &storage.GetObjectRequest{Key: key})
	if err != nil {
		return nil, err
	}

	return pkgaudio.Parse(resp.Body)
}

// cleanupIntermediateAudio 在最终 WAV 合并上传成功后批量删除中间音频，失败仅记日志。
func (s *audioSynthizer) cleanupIntermediateAudio(ctx context.Context, meta *audioCheckpointMeta) {
	if meta == nil || len(meta.Parts) == 0 {
		return
	}
	keys := make([]string, 0, len(meta.Parts))
	for _, p := range meta.Parts {
		keys = append(keys, p.StoreKey)
	}
	if err := s.storage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{
		Keys: keys,
	}); err != nil {
		slog.ErrorContext(ctx, "cleanup intermediate audio failed",
			slog.Int("count", len(keys)),
			slog.Any("err", err),
		)
		return
	}

	slog.InfoContext(ctx, "intermediate audio cleaned up", slog.Int("count", len(keys)))
}

// DiscardStale 清理废弃的中间音频并清空 field3。
// 当 transcript 被重新生成（field2 是新写入的）但 checkpoint.field3 仍有旧数据时，
// 旧音频与新 transcript 不匹配，必须丢弃并从零重新合成。
func (s *audioSynthizer) discardStale(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) {
	meta := restoreAudioMeta(ckpt)
	if meta == nil || len(meta.Parts) == 0 {
		return
	}

	slog.WarnContext(ctx, "discarding stale intermediate audio",
		slog.String("artifact_id", artifactId.String()),
		slog.Int("part_count", len(meta.Parts)),
	)

	s.cleanupIntermediateAudio(ctx, meta)

	ckpt.UpdateField3(nil)
	if err := s.checkpoints.save(ctx, ckpt); err != nil {
		slog.ErrorContext(ctx, "clear stale audio checkpoint failed",
			slog.String("artifact_id", artifactId.String()),
			slog.Any("err", err),
		)
	}
}

// snapshotAudioCheckpoint 序列化 meta 并更新 Field3，返回浅拷贝供锁外保存。
func snapshotAudioCheckpoint(ckpt *workerentity.Checkpoint, meta *audioCheckpointMeta) (*workerentity.Checkpoint, error) {
	if ckpt == nil {
		return nil, errors.New("audio checkpoint is nil")
	}
	data, err := sonic.Marshal(meta)
	if err != nil {
		return nil, err
	}
	ckpt.UpdateField3(data)
	snap := *ckpt
	return &snap, nil
}

// persistAudioCheckpoint 无并发场景下直接序列化并保存 checkpoint（如失败重置路径）。
func (s *audioSynthizer) persistAudioCheckpoint(
	ctx context.Context,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	meta *audioCheckpointMeta,
) error {
	if meta == nil {
		return errors.New("audio checkpoint meta is nil")
	}
	data, err := sonic.Marshal(meta)
	if err != nil {
		return err
	}
	if ckpt == nil {
		if loaded := s.checkpoints.load(ctx, artifactId); loaded != nil {
			ckpt = loaded
		} else {
			ckpt = workerentity.NewCheckpoint(artifactId)
		}
	}
	ckpt.UpdateField3(data)

	return s.checkpoints.save(ctx, ckpt)
}

func restoreAudioMeta(ckpt *workerentity.Checkpoint) *audioCheckpointMeta {
	if ckpt == nil || len(ckpt.Field3) == 0 {
		return &audioCheckpointMeta{}
	}
	var meta audioCheckpointMeta
	if err := sonic.Unmarshal(ckpt.Field3, &meta); err != nil {
		return &audioCheckpointMeta{}
	}

	return &meta
}

func assertOrInitFormat(meta *audioCheckpointMeta, pcm *pkgaudio.PCM) error {
	if pcm == nil {
		return errors.New("pcm is nil")
	}
	if meta.NumChannels == 0 {
		return nil
	}
	if pcm.NumChannels != meta.NumChannels ||
		pcm.SampleRate != meta.SampleRate ||
		pcm.BitsPerSample != meta.BitsPerSample {
		return fmt.Errorf(
			"pcm part format incompatible (ch=%d sr=%d bits=%d) with base (ch=%d sr=%d bits=%d)",
			pcm.NumChannels, pcm.SampleRate, pcm.BitsPerSample,
			meta.NumChannels, meta.SampleRate, meta.BitsPerSample,
		)
	}

	return nil
}

func formatAudioStoreKey(notebookId, artifactId valobj.Id) string {
	return fmt.Sprintf("artifact/%s/%s.wav", notebookId.String(), artifactId.String())
}

// formatIntermediateAudioStoreKey 格式 artifact/{nb}/{art}/audio/turn_{index:06d}.wav
func formatIntermediateAudioStoreKey(notebookId, artifactId valobj.Id, index int) string {
	return fmt.Sprintf("artifact/%s/%s/audio/turn_%06d.wav",
		notebookId.String(), artifactId.String(), index)
}
