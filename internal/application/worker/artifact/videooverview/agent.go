package videooverview

import (
	"github.com/gonotelm-lab/gonotelm/internal/application/worker/artifact/types"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm/chat"
)

func newVideoScriptAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.VideoOverview
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

func newStoryboardAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.VideoOverview
	return types.NewExploreAgentBuilder(deps).
		WithModel(cfg.ModelProvider, cfg.Model).
		WithMaxRound(cfg.MaxRound).
		WithoutBindAllTools().
		WithOptions(
			chat.WithModel(cfg.Model),
			chat.WithThinking(cfg.ModelProvider, false),
			chat.WithMaxTokens(16384),
		).
		Build(req)
}

// newHyperframesAgent 构造沙箱视频渲染 agent（启用 thinking，轮次较长）。
func newHyperframesAgent(deps *types.WorkerDeps, req *types.Request) (*types.Agent, error) {
	cfg := conf.WorkerGlobal().Studio.VideoOverview
	maxRound := cfg.GenerateMaxRound
	if maxRound <= 0 {
		maxRound = cfg.MaxRound
	}
	return types.NewExploreAgentBuilder(deps).
		WithModel(cfg.ModelProvider, cfg.Model).
		WithMaxRound(maxRound).
		WithoutBindAllTools().
		WithOptions(
			chat.WithModel(cfg.Model),
			chat.WithThinking(cfg.ModelProvider, true), // we need think cause generating video is a complex task
		).
		Build(req)
}
