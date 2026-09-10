package videooverview

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

// audioCheckpointMeta 持久化到 checkpoint.field3，记录已上传的逐场景音频元信息，供渲染与断点续跑使用。
type audioCheckpointMeta struct {
	NumChannels   uint16           `json:"num_channels"`
	SampleRate    uint32           `json:"sample_rate"`
	BitsPerSample uint16           `json:"bits_per_sample"`
	Parts         []audioScenePart `json:"parts"`
}

type audioScenePart struct {
	Index      int    `json:"index"`
	StoreKey   string `json:"store_key"`
	DurationMs int64  `json:"duration_ms"`
}

// audioSynthizer 负责按分镜逐场景合成旁白音频：TTS、上传 OSS、断点续跑与陈旧音频清理。
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

func (s *audioSynthizer) generate(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	sb *videoStoryboard,
	ckpt *workerentity.Checkpoint,
) (*audioCheckpointMeta, error) {
	if s.provider == "" {
		return nil, errors.ErrInner.Msgf("audio model provider is empty")
	}

	audioGenerator, err := s.text2audio.GetProvider(s.provider)
	if err != nil {
		return nil, errors.WithMessagef(err, "get text2audio provider failed")
	}

	voice, err := resolveNarratorVoice(s.provider, payload.GetLanguage())
	if err != nil {
		return nil, err
	}

	meta := restoreAudioMeta(ckpt)

	if err := s.synthesizePendingScenes(ctx, req, payload, sb, meta, ckpt, audioGenerator, voice); err != nil {
		return nil, errors.WithMessagef(err, "synthesize pending scenes failed")
	}

	slog.InfoContext(ctx, "video overview scenes audio synthesized",
		slog.String("artifact_id", req.ArtifactId.String()),
		slog.Int("scenes", len(sb.Scenes)),
		slog.Int("done", len(meta.Parts)),
	)

	return meta, nil
}

// synthesizePendingScenes 并发合成未完成的场景旁白，每段成功即上传 OSS 并增量写 checkpoint.field3。
func (s *audioSynthizer) synthesizePendingScenes(
	ctx context.Context,
	req *types.Request,
	payload *artifactentity.VideoOverviewPayload,
	sb *videoStoryboard,
	meta *audioCheckpointMeta,
	ckpt *workerentity.Checkpoint,
	audioGenerator audios.Generator,
	voice string,
) error {
	done := len(meta.Parts)
	if done >= len(sb.Scenes) {
		slog.DebugContext(ctx, "[video] all scenes already synthesized, skip",
			slog.String("artifact_id", req.ArtifactId.String()),
			slog.Int("total", len(sb.Scenes)),
			slog.Int("done", done),
		)
		return nil
	}

	gp, gctx := errgroup.WithContext(ctx)
	gp.SetLimit(s.concurrency)

	var callOpts []audios.Option
	if opt := text2audio.WAVOption(s.provider); opt != nil {
		callOpts = append(callOpts, opt)
	}

	var mu sync.Mutex
	doneSet := make(map[int]bool, len(meta.Parts))
	for _, p := range meta.Parts {
		doneSet[p.Index] = true
	}

	for i := range sb.Scenes {
		if doneSet[i] {
			continue
		}
		gp.Go(func() error {
			scene := &sb.Scenes[i]

			ttsReq := &schema.Request{
				Model:    s.model,
				Text:     scene.Narration,
				Voice:    voice,
				Language: text2audio.AudioLang(string(payload.GetLanguage())),
			}

			resp, err := audioGenerator.Generate(gctx, ttsReq, callOpts...)
			if err != nil {
				return errors.Wrapf(err, "tts generate failed for scene %d (name=%s)", i, scene.Name)
			}

			reader, err := audioutil.ResolveResponse(resp,
				audioutil.WithResolveContext(gctx),
				audioutil.WithResolveHttpClient(s.downloadClient),
			)
			if err != nil {
				return errors.WithMessagef(err, "resolve tts audio for scene %d failed", i)
			}
			defer reader.Close()

			raw, err := io.ReadAll(reader)
			if err != nil {
				return errors.WithMessagef(err, "read tts audio for scene %d failed", i)
			}

			pcm, err := pkgaudio.Parse(raw)
			if err != nil {
				return errors.WithMessagef(err, "parse wav for scene %d failed", i)
			}

			// meta 为并发共享对象，格式初始化也需持锁
			mu.Lock()
			fmtErr := assertOrInitFormat(meta, pcm)
			mu.Unlock()
			if fmtErr != nil {
				return errors.WithMessagef(fmtErr, "format incompatible for scene %d", i)
			}

			partKey := formatSceneAudioStoreKey(req.NotebookId, req.ArtifactId, i)
			if err := s.storage.UploadObject(gctx, &storage.UploadObjectRequest{
				Key:         partKey,
				Body:        raw,
				ContentType: "audio/wav",
			}); err != nil {
				return errors.WithMessagef(err, "upload intermediate audio for scene %d failed", i)
			}

			// 落 checkpoint 必须在 OSS 上传成功之后；锁内只做快照，DB 写入在锁外。
			mu.Lock()
			meta.Parts = append(meta.Parts, audioScenePart{
				Index:      i,
				StoreKey:   partKey,
				DurationMs: pcm.DurationMs(),
			})
			snap, snapErr := snapshotAudioCheckpoint(ckpt, meta)
			mu.Unlock()

			if snapErr != nil {
				slog.WarnContext(gctx, "snapshot audio checkpoint failed",
					slog.String("artifact_id", req.ArtifactId.String()),
					slog.Any("err", snapErr))
			} else if saveErr := s.checkpoints.save(gctx, snap); saveErr != nil {
				slog.WarnContext(gctx, "persist audio checkpoint failed",
					slog.String("artifact_id", req.ArtifactId.String()),
					slog.Any("err", saveErr))
			}

			slog.DebugContext(gctx, "video scene audio synthesized",
				slog.String("artifact_id", req.ArtifactId.String()),
				slog.Int("scene", i),
				slog.String("name", scene.Name),
				slog.Int("pcm_bytes", len(pcm.Data)),
			)
			return nil
		})
	}

	return gp.Wait()
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

// DiscardStale 分镜重新生成后旧音频作废：删除中间音频并清空 field3。
func (s *audioSynthizer) discardStale(ctx context.Context, ckpt *workerentity.Checkpoint) {
	meta := restoreAudioMeta(ckpt)
	if meta == nil || len(meta.Parts) == 0 {
		return
	}

	keys := make([]string, 0, len(meta.Parts))
	for _, p := range meta.Parts {
		keys = append(keys, p.StoreKey)
	}

	slog.WarnContext(ctx, "discarding stale video scene audio", slog.Int("part_count", len(keys)))

	s.deleteParts(ctx, keys)

	ckpt.UpdateField3(nil)
	if err := s.checkpoints.save(ctx, ckpt); err != nil {
		slog.ErrorContext(ctx, "clear stale audio checkpoint failed", slog.Any("err", err))
	}
}

// DiscardStaleStoryboard 大纲重新生成后，旧分镜与旧音频全部作废。
func (s *audioSynthizer) discardStaleStoryboard(ctx context.Context, ckpt *workerentity.Checkpoint) {
	if len(ckpt.Field2) == 0 && len(ckpt.Field3) == 0 {
		return
	}

	if meta := restoreAudioMeta(ckpt); meta != nil && len(meta.Parts) > 0 {
		keys := make([]string, 0, len(meta.Parts))
		for _, p := range meta.Parts {
			keys = append(keys, p.StoreKey)
		}
		slog.WarnContext(ctx, "discarding stale video storyboard and audio", slog.Int("part_count", len(keys)))
		s.deleteParts(ctx, keys)
	} else {
		slog.WarnContext(ctx, "discarding stale video storyboard")
	}

	ckpt.UpdateField2(nil)
	ckpt.UpdateField3(nil)
	if err := s.checkpoints.save(ctx, ckpt); err != nil {
		slog.ErrorContext(ctx, "clear stale storyboard/audio checkpoint failed", slog.Any("err", err))
	}
}

func (s *audioSynthizer) deleteParts(ctx context.Context, keys []string) {
	if len(keys) == 0 {
		return
	}
	if err := s.storage.BatchDeleteObject(ctx, &storage.BatchDeleteObjectRequest{Keys: keys}); err != nil {
		slog.ErrorContext(ctx, "cleanup stale video audio failed", slog.Int("count", len(keys)), slog.Any("err", err))
	}
}

func assertOrInitFormat(meta *audioCheckpointMeta, pcm *pkgaudio.PCM) error {
	if pcm == nil {
		return errors.New("pcm is nil")
	}
	if meta.NumChannels == 0 {
		meta.NumChannels = pcm.NumChannels
		meta.SampleRate = pcm.SampleRate
		meta.BitsPerSample = pcm.BitsPerSample
		return nil
	}
	if pcm.NumChannels != meta.NumChannels ||
		pcm.SampleRate != meta.SampleRate ||
		pcm.BitsPerSample != meta.BitsPerSample {
		return errors.Errorf(
			"pcm part format incompatible (ch=%d sr=%d bits=%d) with base (ch=%d sr=%d bits=%d)",
			pcm.NumChannels, pcm.SampleRate, pcm.BitsPerSample,
			meta.NumChannels, meta.SampleRate, meta.BitsPerSample,
		)
	}
	return nil
}

// formatSceneAudioStoreKey 格式 artifact/{nb}/{art}/video/audio/scene_{index:06d}.wav
func formatSceneAudioStoreKey(notebookId, artifactId valobj.Id, index int) string {
	return fmt.Sprintf("artifact/%s/%s/video/audio/scene_%06d.wav",
		notebookId.String(), artifactId.String(), index)
}
