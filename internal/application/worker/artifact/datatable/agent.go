package datatable

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

// newDataTableAgent 构造 datatable 步骤的 source explore agent。
func newDataTableAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.DataTable
	return types.NewExploreAgentBuilder(deps).
		WithModel(cfg.ModelProvider, cfg.Model).
		WithMaxRound(cfg.MaxRound).
		WithOptions(
			chat.WithModel(cfg.Model),
		).
		Build(req)
}
