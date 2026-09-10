package videooverview

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Generator 编排视频产物的各生成步骤：大纲 → 分镜 → 逐场景旁白音频。
type Generator struct {
	checkpoints *checkpointStore
	outline     *outlineGenerator
	storyboard  *storyboardGenerator
	audio       *audioSynthizer
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	checkpoints := newCheckpointStore(deps.CheckpointRepository)
	agents := newVideoAgentFactory(deps)
	return &Generator{
		checkpoints: checkpoints,
		outline:     newOutlineGenerator(agents, checkpoints),
		storyboard:  newStoryboardGenerator(agents, checkpoints),
		audio:       newAudioSynthizer(deps, checkpoints),
	}
}

// Generate 流程：大纲（field1）→ 分镜（field2）→ 逐场景旁白音频（field3）。
// 视频渲染未实现，返回错误终态；checkpoint 已持久化，补齐渲染后可基于 checkpoint 续跑。
func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	payload := artifactentity.PayloadAs[*artifactentity.VideoOverviewPayload](req.Payload)

	ckpt := g.checkpoints.load(ctx, req.ArtifactId)

	outline, ckpt, outlineRestored, err := g.outline.ensure(ctx, req, payload, ckpt)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate video outline failed")
	}

	if !outlineRestored {
		g.audio.discardStaleStoryboard(ctx, ckpt)
	}

	storyboard, ckpt, storyboardRestored, err := g.storyboard.ensure(ctx, req, payload, outline, ckpt)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate video storyboard failed")
	}

	if !storyboardRestored {
		g.audio.discardStale(ctx, ckpt)
	}

	if _, err := g.audio.generate(ctx, req, payload, storyboard, ckpt); err != nil {
		return nil, errors.WithMessagef(err, "generate video overview audio failed")
	}

	// TODO: 渲染视频（HTML composition）尚未实现。
	return nil, errors.ErrParams.Msgf("video_overview rendering is not implemented yet")
}
