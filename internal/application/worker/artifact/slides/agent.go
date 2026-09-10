package slides

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

// newSlidesOutlineAgent 构造大纲步骤的 agent（JSON 输出，关闭 thinking）。
func newSlidesOutlineAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.Slides
	return types.NewExploreAgentBuilder(deps).
		WithModel(cfg.ModelProvider, cfg.Model).
		WithMaxRound(cfg.MaxRound).
		WithOptions(
			chat.WithModel(cfg.Model),
			chat.WithResponseJsonObject(cfg.ModelProvider),
			chat.WithThinking(cfg.ModelProvider, false),
		).
		Build(req)
}

// newSlidesPPTXAgent 构造 PPTX 步骤的 agent（启用 thinking，生成耗时较长）。
func newSlidesPPTXAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.Slides
	return types.NewExploreAgentBuilder(deps).
		WithModel(cfg.ModelProvider, cfg.Model).
		WithMaxRound(cfg.GenerateMaxRound).
		WithOptions(
			chat.WithModel(cfg.Model),
			chat.WithThinking(cfg.ModelProvider, true),
		).
		Build(req)
}
