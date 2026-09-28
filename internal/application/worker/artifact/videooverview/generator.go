package videooverview

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	workerentity "github.com/gonotelm-lab/gonotelm/internal/domain/worker/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/pipeline"
)

// 步骤之间通过 pipeline.Data 传递的键。
const (
	dataKeyCheckpoint     = "videooverview.checkpoint"
	dataKeyScript         = "videooverview.script"
	dataKeyScriptRestored = "videooverview.script_restored"
	dataKeyAudioMeta      = "videooverview.audio_meta"
	dataKeyAudioRestored  = "videooverview.audio_restored"
	dataKeyStoryboard     = "videooverview.storyboard"
	dataKeyResult         = "videooverview.result"
)

// Generator 用 pipeline 编排视频产物：口播稿（field1）→ 逐句旁白音频（field2）→ 分镜 Markdown（field3）→ 沙箱渲染 MP4。
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

	p := pipeline.New("worker.videooverview")
	p.AddSteps(
		&scriptStep{
			deps:          g.deps,
			checkpoints:   g.checkpoints,
			audioProvider: conf.WorkerGlobal().Studio.VideoOverview.AudioModelProvider,
		},
		newAudioStep(g.deps, g.checkpoints),
		&storyboardStep{deps: g.deps, checkpoints: g.checkpoints},
		newHyperframesStep(g.deps),
	)

	if err := p.Execute(ctx, data); err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(pipeline.Get[*videoStorageResult](data, dataKeyResult))
	if err != nil {
		return nil, errors.Wrap(err, "marshal video overview result failed")
	}

	return &types.Response{
		Title:      scriptFrom(data).Title,
		Result:     result,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}

func payloadFrom(req *types.Request) *artifactentity.VideoOverviewPayload {
	return artifactentity.PayloadAs[*artifactentity.VideoOverviewPayload](req.Payload)
}

func checkpointFrom(data *pipeline.Data) *workerentity.Checkpoint {
	return pipeline.Get[*workerentity.Checkpoint](data, dataKeyCheckpoint)
}

func scriptFrom(data *pipeline.Data) *videoScript {
	return pipeline.Get[*videoScript](data, dataKeyScript)
}

func audioMetaFrom(data *pipeline.Data) *audioCheckpointMeta {
	return pipeline.Get[*audioCheckpointMeta](data, dataKeyAudioMeta)
}

func storyboardFrom(data *pipeline.Data) string {
	return pipeline.Get[string](data, dataKeyStoryboard)
}
