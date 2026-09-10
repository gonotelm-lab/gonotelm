package audiooverview

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"

	einomodel "github.com/cloudwego/eino/components/model"
)

// newAudioAgentFactory 用 audioOverview 的模型配置构造通用 agent 工厂。
func newAudioAgentFactory(deps *types.WorkerDeps) *types.AgentFactory {
	cfg := conf.WorkerGlobal().Studio.AudioOverview
	options := []einomodel.Option{
		chat.WithModel(cfg.Model),
		chat.WithResponseJsonObject(cfg.ModelProvider),
		chat.WithThinking(cfg.ModelProvider, false),
	}
	return types.NewAgentFactory(deps, cfg.ModelProvider, cfg.Model, cfg.MaxRound, options)
}
