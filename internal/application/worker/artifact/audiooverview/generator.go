package audiooverview

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
)

// 步骤之间通过 pipeline.Data 传递的键。
const (
	dataKeyCheckpoint         = "audiooverview.checkpoint"
	dataKeyOutline            = "audiooverview.outline"
	dataKeyTranscript         = "audiooverview.transcript"
	dataKeyTranscriptRestored = "audiooverview.transcript_restored"
	dataKeyAudioResult        = "audiooverview.audio_result"
)

// Generator 用 pipeline 编排播客产物：大纲（field1）→ 文字稿（field2）→ 逐段合成并拼接音频（field3）。
type Generator struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	return &Generator{
		deps:        deps,
		checkpoints: types.NewCheckpointStore(deps.CheckpointRepository),
	}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	data := types.NewPipelineData(req)
	data.Set(dataKeyCheckpoint, g.checkpoints.Load(ctx, req.ArtifactId))

	p := pipeline.New("worker.audiooverview")
	p.AddSteps(
		&outlineStep{deps: g.deps, checkpoints: g.checkpoints},
		&transcriptStep{
			deps:          g.deps,
			checkpoints:   g.checkpoints,
			audioProvider: conf.WorkerGlobal().Studio.AudioOverview.AudioModelProvider,
		},
		newAudioStep(g.deps, g.checkpoints),
	)

	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(pipeline.Get[*AudioStorageResult](data, dataKeyAudioResult))
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal podcast audio result err=%v", err)
	}

	return &types.Response{
		Title:      pipeline.Get[*podcastTranscriptExpectation](data, dataKeyTranscript).Title,
		Result:     result,
		ResultKind: entity.ResultKindStorage,
	}, nil
}

func payloadFrom(req *types.Request) *entity.AudioOverviewPayload {
	return entity.PayloadAs[*entity.AudioOverviewPayload](req.Payload)
}

func checkpointFrom(data *pipeline.Data) *workerentity.Checkpoint {
	return pipeline.Get[*workerentity.Checkpoint](data, dataKeyCheckpoint)
}

func outlineFrom(data *pipeline.Data) *podcastOutlineExpectation {
	return pipeline.Get[*podcastOutlineExpectation](data, dataKeyOutline)
}

func transcriptFrom(data *pipeline.Data) *podcastTranscriptExpectation {
	return pipeline.Get[*podcastTranscriptExpectation](data, dataKeyTranscript)
}
