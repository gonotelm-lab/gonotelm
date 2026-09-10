package audiooverview

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Generator 编排播客产物的各生成步骤：大纲 → 文字稿 → 音频合成。
type Generator struct {
	checkpoints *checkpointStore
	outline     *outlineGenerator
	transcript  *transcriptGenerator
	audio       *audioSynthizer
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	checkpoints := newCheckpointStore(deps.CheckpointRepository)
	return &Generator{
		checkpoints: checkpoints,
		outline:     newOutlineGenerator(deps, checkpoints),
		transcript:  newTranscriptGenerator(deps, checkpoints, conf.WorkerGlobal().Studio.AudioOverview.AudioModelProvider),
		audio:       newAudioSynthizer(deps, checkpoints),
	}
}

// Generate 流程：大纲（field1）→ 文字稿（field2）→ 逐段合成并拼接音频（field3）。
func (a *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	payload := entity.PayloadAs[*entity.AudioOverviewPayload](req.Payload)

	ckpt := a.checkpoints.load(ctx, req.ArtifactId)

	outline, ckpt, err := a.outline.ensure(ctx, req, payload, ckpt)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate outline failed")
	}

	transcript, ckpt, transcriptRestored, err := a.transcript.ensure(ctx, req, payload, ckpt, outline)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate transcript failed")
	}

	if !transcriptRestored && ckpt != nil && ckpt.Field3 != nil {
		a.audio.discardStale(ctx, req.ArtifactId, ckpt)
	}

	audioResult, err := a.audio.generate(ctx, req, payload, transcript, ckpt)
	if err != nil {
		slog.ErrorContext(ctx, "generate audio failed",
			slog.String("artifact_id", req.ArtifactId.String()),
			slog.String("notebook_id", payload.NotebookId.String()),
			slog.String("style", string(payload.Style)),
			slog.Any("err", err),
		)
		return nil, errors.WithMessagef(err, "generate audio failed")
	}

	return a.buildAudioResponse(transcript, audioResult)
}

func (a *Generator) buildAudioResponse(
	transcript *podcastTranscriptExpectation,
	audioResult *AudioStorageResult,
) (*types.Response, error) {
	result, err := sonic.Marshal(audioResult)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal podcast audio result err=%v", err)
	}
	return &types.Response{
		Title:      transcript.Title,
		Result:     result,
		ResultKind: entity.ResultKindStorage,
	}, nil
}
