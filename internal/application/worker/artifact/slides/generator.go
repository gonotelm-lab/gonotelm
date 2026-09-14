package slides

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"

	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Generator 编排幻灯片产物：大纲（field1）→ 准备沙箱 → 沙箱 agent 生成 PPTX 并上传。
type Generator struct {
	deps        *types.WorkerDeps
	checkpoints *types.CheckpointStore
	outline     *outlineGenerator
	sandbox     *sandboxProvider
	pptx        *pptxGenerator
}

var _ types.Generator = &Generator{}

func New(deps *types.WorkerDeps) *Generator {
	checkpoints := types.NewCheckpointStore(deps.CheckpointRepository)
	return &Generator{
		deps:        deps,
		checkpoints: checkpoints,
		outline:     newOutlineGenerator(deps, checkpoints),
		sandbox:     newSandboxProvider(deps),
		pptx:        newPPTXGenerator(deps),
	}
}

func (g *Generator) Generate(ctx context.Context, req *types.Request) (*types.Response, error) {
	ckpt := g.checkpoints.Load(ctx, req.ArtifactId)

	sources, err := loadOutlineSources(ctx, g.deps, req.SourceIds)
	if err != nil {
		return nil, errors.WithMessage(err, "load outline sources failed")
	}

	outlineExp, _, err := g.outline.ensure(ctx, req, sources, ckpt)
	if err != nil {
		return nil, errors.WithMessage(err, "ensure outline failed")
	}

	slog.DebugContext(ctx, "slides outline generated", slog.String("notebook_id", req.NotebookId.String()),
		slog.String("artifact_id", req.ArtifactId.String()),
	)

	sandbox, err := g.sandbox.ensure(ctx, req)
	if err != nil {
		return nil, errors.WithMessagef(err, "ensure sandbox failed")
	}

	slog.DebugContext(ctx, "slides generation ensure sandbox done")

	result, err := g.pptx.generate(ctx, req, outlineExp, sandbox, sources)
	if err != nil {
		return nil, errors.WithMessage(err, "generate slides failed")
	}

	slog.DebugContext(ctx, "slides generation pptx done")

	resultBytes, err := sonic.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "marshal slides generation result failed")
	}

	return &types.Response{
		Title:      outlineExp.Title,
		Result:     resultBytes,
		ResultKind: artifactentity.ResultKindStorage,
	}, nil
}
