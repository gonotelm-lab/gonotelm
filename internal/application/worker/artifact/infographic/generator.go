package infographic

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Generator 编排信息图产物：文生图 prompt（field1）→ 生成并存储图片。
type Generator struct {
	checkpoints *types.CheckpointStore
	prompt      *imagePromptGenerator
	image       *imageGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	checkpoints := types.NewCheckpointStore(deps.CheckpointRepository)
	return &Generator{
		checkpoints: checkpoints,
		prompt:      newImagePromptGenerator(deps, checkpoints),
		image:       newImageGenerator(deps),
	}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	payload := artifactentity.PayloadAs[*artifactentity.InfoGraphicPayload](req.Payload)

	ckpt := g.checkpoints.Load(ctx, req.ArtifactId)

	expect, _, err := g.prompt.ensure(ctx, req, payload, ckpt)
	if err != nil {
		return nil, err
	}

	slog.DebugContext(ctx, "generate infographic expectation done, now generate image",
		slog.String("task_id", req.ArtifactId.String()),
		slog.String("title", expect.Title),
	)

	storageResult, err := g.image.generate(ctx, req.ArtifactId, payload, expect.ImagePrompt)
	if err != nil {
		return nil, err
	}

	result, err := sonic.Marshal(storageResult)
	if err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "marshal infographic storage result err=%v", err)
	}

	return &types.Response{
		Title:      expect.Title,
		Result:     result,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}
