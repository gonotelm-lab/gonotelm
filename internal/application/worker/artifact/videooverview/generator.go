package videooverview

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Generator 编排视频产物：口播稿 → 逐句 TTS → 分镜脚本 → HyperFrames 渲染。
type Generator struct {
	checkpoints *checkpointStore
	script      *scriptGenerator
	audio       *audioSynthizer
	storyboard  *storyboardGenerator
	hyperframes *hyperframesVideoGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	checkpoints := newCheckpointStore(deps.CheckpointRepository)
	return &Generator{
		checkpoints: checkpoints,
		script:      newScriptGenerator(deps, checkpoints),
		audio:       newAudioSynthizer(deps, checkpoints),
		storyboard:  newStoryboardGenerator(deps, checkpoints),
		hyperframes: newHyperframesVideoGenerator(deps),
	}
}

// Generate 流程：口播稿（field1）→ 逐句旁白音频（field2）→ 分镜 Markdown（field3）→ 沙箱渲染 MP4。
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

	slog.DebugContext(ctx, "video overview audio ready, start storyboard generation",
		slog.String("artifact_id", req.ArtifactId.String()),
	)

	storyboardMD, _, _, err := g.storyboard.ensure(ctx, req, payload, script, audioMeta, ckpt)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate video storyboard failed")
	}

	slog.DebugContext(ctx, "video overview storyboard ready, start hyperframes render",
		slog.String("artifact_id", req.ArtifactId.String()),
	)

	result, err := g.hyperframes.generate(ctx, req, payload, storyboardMD, audioMeta)
	if err != nil {
		return nil, errors.WithMessagef(err, "generate hyperframes video failed")
	}

	resultBytes, err := sonic.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "marshal video overview result failed")
	}

	return &types.Response{
		Title:      script.Title,
		Result:     resultBytes,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}
