package videooverview

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Generator 编排视频产物：口播稿 → 逐句 TTS → 分镜脚本。
type Generator struct {
	checkpoints *checkpointStore
	script      *scriptGenerator
	audio       *audioSynthizer
	storyboard  *storyboardGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	checkpoints := newCheckpointStore(deps.CheckpointRepository)
	return &Generator{
		checkpoints: checkpoints,
		script:      newScriptGenerator(deps, checkpoints),
		audio:       newAudioSynthizer(deps, checkpoints),
		storyboard:  newStoryboardGenerator(deps, checkpoints),
	}
}

// Generate 流程：口播稿（field1）→ 逐句旁白音频（field2）→ 分镜 Markdown（field3）。
// 视频渲染尚未实现。
func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	payload := artifactentity.PayloadAs[*artifactentity.VideoOverviewPayload](req.Payload)

	ckpt := g.checkpoints.load(ctx, req.ArtifactId)

	script, ckpt, scriptRestored, err := g.script.ensure(ctx, req, payload, ckpt)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate video script failed")
	}

	if !scriptRestored {
		g.audio.discardStale(ctx, req.ArtifactId, ckpt)
		g.storyboard.discardStale(ctx, req.ArtifactId, ckpt)
	}

	audioMeta, audioRestored, err := g.audio.generate(ctx, req, payload, script, ckpt)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate video overview audio failed")
	}

	if !audioRestored {
		g.storyboard.discardStale(ctx, req.ArtifactId, ckpt)
	}

	if _, _, _, err := g.storyboard.ensure(ctx, req, payload, script, audioMeta, ckpt); err != nil {
		return nil, errors.WithMessagef(err, "generate video storyboard failed")
	}

	// TODO: 渲染视频尚未实现。
	return nil, errors.ErrParams.Msgf("video_overview rendering is not implemented yet")
}
