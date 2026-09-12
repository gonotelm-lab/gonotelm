package infographic

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

// newInfoGraphicAgent 构造 infographic 步骤的 source explore agent。
// bindAllTools 关闭时仅绑定 StatSource/GrepSource/QuerySource，用于简洁模式。
func newInfoGraphicAgent(deps *types.WorkerDeps, req *types.Request, bindAllTools bool) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.InfoGraphic
	builder := types.NewExploreAgentBuilder(deps).
		WithModel(cfg.ModelProvider, cfg.Model).
		WithMaxRound(cfg.MaxRound).
		WithOptions(chat.WithModel(cfg.Model))
	if !bindAllTools {
		builder = builder.WithoutBindAllTools()
	}
	return builder.Build(req)
}
