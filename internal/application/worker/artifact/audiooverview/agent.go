package audiooverview

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

// newAudioAgent 构造 audiooverview 步骤的 source explore agent。
func newAudioAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.AudioOverview
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
