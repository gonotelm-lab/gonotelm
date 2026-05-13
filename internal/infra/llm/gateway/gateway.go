package gateway

import (
	"context"
	"fmt"
	"sync"

	"github.com/gonotelm-lab/gonotelm/internal/infra/llm/chat"

	einomodel "github.com/cloudwego/eino/components/model"
)

// 管理项目中的多个提供商的LLM模型，根据配置选择不同的模型
// 
// TODO 可以在此Gateway中统一管控模型：比如模型动态增减、token消耗记数、监控追踪等
type Gateway struct {
	mu sync.RWMutex

	providers map[chat.Type]einomodel.ToolCallingChatModel
}

func New(cfg *chat.ProviderConfig) (*Gateway, error) {
	gw := &Gateway{
		providers: make(map[chat.Type]einomodel.ToolCallingChatModel),
	}

	// 初始化模型实例
	err := gw.initProviders(cfg)
	if err != nil {
		return nil, err
	}

	return gw, nil
}

func (g *Gateway) initProviders(cfg *chat.ProviderConfig) error {
	ctx := context.Background()
	// 1. deepseek
	deepseekModel, err := chat.New(ctx, chat.DeepSeek, cfg)
	if err != nil {
		return err
	}
	g.providers[chat.DeepSeek] = deepseekModel

	// 2. openai
	openaiModel, err := chat.New(ctx, chat.Openai, cfg)
	if err != nil {
		return err
	}
	g.providers[chat.Openai] = openaiModel

	// 3. qwen

	qwenModel, err := chat.New(ctx, chat.Qwen, cfg)
	if err != nil {
		return err
	}
	g.providers[chat.Qwen] = qwenModel

	return nil
}

func (g *Gateway) GetProvider(
	providerType chat.Type,
) (einomodel.ToolCallingChatModel, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	provider, ok := g.providers[providerType]
	if !ok {
		return nil, fmt.Errorf("provider %s not found", providerType)
	}

	return provider, nil
}
