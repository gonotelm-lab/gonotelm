package audiooverview

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/bytedance/sonic"
	audios "github.com/gonotelm-lab/multimodal/audio"
	mimopkg "github.com/gonotelm-lab/multimodal/audio/mimo"
	minimaxpkg "github.com/gonotelm-lab/multimodal/audio/minimax"
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

func wavOptionForProvider(provider text2audio.Text2AudioProvider) audios.Option {
	switch provider {
	case text2audio.Text2AudioDashScope:
		// dashscope 已默认返回 WAV
		return nil
	case text2audio.Text2AudioMimo:
		return mimopkg.WithFormat(mimopkg.FormatWAV)
	case text2audio.Text2AudioMiniMax:
		return minimaxpkg.WithAudioFormat(minimaxpkg.AudioFormatWAV)
	}

	return nil
}

// generateAudio 逐段调用 TTS 并上传中间 WAV 到 OSS，每段成功后立即写 checkpoint.Field3。
// 重试时跳过已合成 index、稀疏补齐失败的 index，最后下载/拼接、上传最终 WAV 并清理中间键。
func toAudioLang(l artifactentity.Language) schema.Language {
	switch l {
	case artifactentity.LanguageChinese:
		return schema.LanguageChinese
	case artifactentity.LanguageEnglish:
		return schema.LanguageEnglish
	default:
		return schema.LanguageAuto
	}
}

func (a *Generator) generateAudio(
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

	cfg := conf.WorkerGlobal().Studio.AudioOverview

	provider := cfg.AudioModelProvider
	if provider == "" {
		return nil, errors.ErrInner.Msgf("audio model provider is empty")
	}

	slog.DebugContext(ctx, "[audio] provider config",
		slog.String("provider", provider.String()),
		slog.String("model", cfg.AudioModel),
	)

	audioGenerator, err := a.deps.Text2Audio.GetProvider(provider)
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

	if err = a.synthesizePendingTurns(ctx,
		payload,
		turns,
		meta,
		req.ArtifactId,
		ckpt,
		audioGenerator,
		voiceByID,
		provider,
		cfg.AudioModel,
	); err != nil {
		return nil, errors.WithMessagef(err, "synthesize pending turns failed")
	}

	slog.DebugContext(ctx, "[audio] assembling ordered PCMs",
		slog.Int("total_turns", len(turns)),
	)

	orderedPCMs, err := a.assembleOrderedPCMs(ctx, len(turns), meta)
	if err != nil {
		slog.WarnContext(ctx, "assemble ordered pcms failed, resetting checkpoint and re-synthesizing",
			slog.String("artifact_id", req.ArtifactId.String()),
			slog.Any("err", err),
		)
		meta.Parts = nil
		meta.NumChannels = 0
		meta.SampleRate = 0
		meta.BitsPerSample = 0
		if err := a.persistAudioCheckpoint(ctx, req.ArtifactId, ckpt, meta); err != nil {
			slog.ErrorContext(ctx, "reset audio checkpoint failed",
				slog.String("artifact_id", req.ArtifactId.String()),
				slog.Any("err", err),
			)
		}
		if err = a.synthesizePendingTurns(ctx,
			payload,
			turns,
			meta,
			req.ArtifactId,
			ckpt,
			audioGenerator,
			voiceByID,
			provider,
			cfg.AudioModel,
		); err != nil {
			return nil, errors.WithMessagef(err, "re-synthesize pending turns failed")
		}
		orderedPCMs, err = a.assembleOrderedPCMs(ctx, len(turns), meta)
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
	if err = a.deps.ObjectStorage.UploadObject(ctx, &storage.UploadObjectRequest{
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
	a.cleanupIntermediateAudio(ctx, meta)

	// 计算 PCM 时长：size / (sample_rate * channels * bits/8) * 1000 ms
	durationMs := int64(0)
	if merged.SampleRate > 0 && merged.NumChannels > 0 && merged.BitsPerSample > 0 {
		bytesPerSecond := uint32(merged.SampleRate) *
			uint32(merged.NumChannels) *
			uint32(merged.BitsPerSample) / 8
		if bytesPerSecond > 0 {
			durationMs = int64(float64(len(merged.Data)) / float64(bytesPerSecond) * 1000)
		}
	}

	return &AudioStorageResult{
		StoreKey:    storeKey,
		ContentType: "audio/wav",
		Audio: &AudioStorageResultMeta{
			Format:        "wav",
			NumChannels:   int(merged.NumChannels),
			SampleRate:    int(merged.SampleRate),
			BitsPerSample: int(merged.BitsPerSample),
			Size:          len(wavBytes),
			DurationMs:    durationMs,
		},
	}, nil
}

// synthesizePendingTurns 并发执行尚未出现在 meta.Parts 中的 turn 的 TTS。
// 每段成功后立即把 WAV 上传至 OSS 并增量写 checkpoint.Field3；任一失败终止剩余任务。
func (a *Generator) synthesizePendingTurns(
	ctx context.Context,
	payload *artifactentity.AudioOverviewPayload,
	turns []synthesizedTurn,
	meta *audioCheckpointMeta,
	artifactId valobj.Id,
	ckpt *workerentity.Checkpoint,
	audioGenerator audios.Generator,
	voiceByID map[string]string,
	provider text2audio.Text2AudioProvider,
	model string,
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
		slog.String("provider", provider.String()),
		slog.String("model", model),
		slog.String("language", string(payload.Language)),
	)

	g, gctx := errgroup.WithContext(ctx)
	concurrency := conf.WorkerGlobal().Studio.AudioOverview.AudioSynthConcurrency
	if concurrency <= 0 {
		concurrency = 1
	}
	g.SetLimit(concurrency)

	var callOpts []audios.Option
	if opt := wavOptionForProvider(provider); opt != nil {
		callOpts = append(callOpts, opt)
	}

	// protected 内 meta + ckpt 的增量读写：每段成功立刻持久化 checkpoint。
	var mu sync.Mutex

	doneSet := make(map[int]bool, len(meta.Parts))
	for _, p := range meta.Parts {
		doneSet[p.Index] = true
	}

	for i := range turns {
		i := i
		if doneSet[i] {
			continue
		}
		g.Go(func() error {
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
				Model:       model,
				Text:        turn.Text,
				Voice:       voice,
				Language:    toAudioLang(payload.Language),
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
				return errors.Wrapf(err,
					"tts generate failed for turn %d (speaker=%s)",
					i, turn.SpeakerName,
				)
			}

			reader, err := audioutil.ResolveResponse(resp, audioutil.WithResolveContext(gctx))
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn resolve response failed",
					slog.Int("turn_index", i),
					slog.Any("err", err),
				)
				return errors.WithMessagef(err,
					"resolve tts audio for turn %d failed", i,
				)
			}
			defer reader.Close()

			raw, err := io.ReadAll(reader)
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn read audio failed",
					slog.Int("turn_index", i),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner,
					"read tts audio for turn %d failed, err=%v", i, err,
				)
			}

			pcm, err := pkgaudio.Parse(raw)
			if err != nil {
				slog.ErrorContext(gctx, "[audio] turn parse wav failed",
					slog.Int("turn_index", i),
					slog.Int("audio_bytes", len(raw)),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner,
					"parse wav for turn %d failed, err=%v", i, err,
				)
			}

			if err = assertOrInitFormat(meta, pcm); err != nil {
				slog.ErrorContext(gctx, "[audio] turn format incompatible",
					slog.Int("turn_index", i),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner,
					"format incompatible for turn %d, err=%v", i, err,
				)
			}

			partKey := formatIntermediateAudioStoreKey(payload.NotebookId, artifactId, i)
			if err = a.deps.ObjectStorage.UploadObject(gctx, &storage.UploadObjectRequest{
				Key:         partKey,
				Body:        raw,
				ContentType: "audio/wav",
			}); err != nil {
				slog.ErrorContext(gctx, "[audio] turn upload intermediate audio failed",
					slog.Int("turn_index", i),
					slog.String("part_key", partKey),
					slog.Any("err", err),
				)
				return errors.Wrapf(errors.ErrInner,
					"upload intermediate audio for turn %d failed, err=%v", i, err,
				)
			}

			// 落 checkpoint 必须在 OSS 上传成功之后；保证 store_key 就一定有音频。
			mu.Lock()
			if meta.NumChannels == 0 {
				meta.NumChannels = pcm.NumChannels
				meta.SampleRate = pcm.SampleRate
				meta.BitsPerSample = pcm.BitsPerSample
			}
			meta.Parts = append(meta.Parts, audioTurnPart{Index: i, StoreKey: partKey})
			mu.Unlock()

			if saveErr := a.persistAudioCheckpoint(gctx, artifactId, ckpt, meta); saveErr != nil {
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

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}

// assembleOrderedPCMs 按 turn index 从 OSS 读回所有逐段 PCM，形成 [0..N-1] 保序切片。
func (a *Generator) assembleOrderedPCMs(
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
	for i := 0; i < total; i++ {
		key, ok := partsByIndex[i]
		if !ok {
			return nil, errors.ErrInner.Msgf("missing intermediate audio for turn %d", i)
		}
		pcm, err := a.downloadTurnPCM(ctx, key)
		if err != nil {
			return nil, errors.WithMessagef(err,
				"download intermediate audio for turn %d failed", i,
			)
		}
		out[i] = pcm
	}
	return out, nil
}

// downloadTurnPCM 从 OSS 下载逐段 WAV 并解析为 PCM。
func (a *Generator) downloadTurnPCM(ctx context.Context, key string) (*pkgaudio.PCM, error) {
	resp, err := a.deps.ObjectStorage.GetObject(ctx, &storage.GetObjectRequest{Key: key})
	if err != nil {
		return nil, err
	}

	return pkgaudio.Parse(resp.Body)
}

// cleanupIntermediateAudio 在最终 WAV 合并上传成功后批量删除中间音频，失败仅记日志。
func (a *Generator) cleanupIntermediateAudio(ctx context.Context, meta *audioCheckpointMeta) {
	if meta == nil || len(meta.Parts) == 0 {
		return
	}
	keys := make([]string, 0, len(meta.Parts))
	for _, p := range meta.Parts {
		keys = append(keys, p.StoreKey)
	}
	if err := a.deps.ObjectStorage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{
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

// discardStaleAudio 清理废弃的中间音频并清空 field3。
// 当 transcript 被重新生成（field2 是新写入的）但 checkpoint.field3 仍有旧数据时，
// 旧音频与新 transcript 不匹配，必须丢弃并从零重新合成。
func (a *Generator) discardStaleAudio(ctx context.Context, artifactId valobj.Id, ckpt *workerentity.Checkpoint) {
	meta := restoreAudioMeta(ckpt)
	if meta == nil || len(meta.Parts) == 0 {
		return
	}

	slog.WarnContext(ctx, "discarding stale intermediate audio",
		slog.String("artifact_id", artifactId.String()),
		slog.Int("part_count", len(meta.Parts)),
	)

	a.cleanupIntermediateAudio(ctx, meta)

	ckpt.UpdateField3(nil)
	if err := a.deps.CheckpointRepository.Save(ctx, ckpt); err != nil {
		slog.ErrorContext(ctx, "clear stale audio checkpoint failed",
			slog.String("artifact_id", artifactId.String()),
			slog.Any("err", err),
		)
	}
}

func (a *Generator) persistAudioCheckpoint(
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
		loaded, loadErr := a.deps.CheckpointRepository.FindByArtifactId(ctx, artifactId)
		if loadErr == nil && loaded != nil {
			ckpt = loaded
		} else {
			ckpt = workerentity.NewCheckpoint(artifactId)
		}
	}
	ckpt.UpdateField3(data)

	return a.deps.CheckpointRepository.Save(ctx, ckpt)
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
