package chat

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/llm"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
)

func NewChatModel(
	ctx context.Context,
	providerType llm.Provider,
	cfg *llm.ProviderConfig,
) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}

	switch providerType {
	case llm.ProviderDeepSeek:
		return deepseek.NewChatModel(ctx, cfg.DeepSeek.ToEino())
	case llm.ProviderOpenAI:
		return openai.NewChatModel(ctx, cfg.OpenAI.ToEino())
	case llm.ProviderQwen:
		return qwen.NewChatModel(ctx, cfg.Qwen.ToEino())
	case llm.ProviderAgnes:
		return agnes.NewChatModel(ctx, cfg.Agnes.ToEino())
	default:
		return nil, fmt.Errorf("model type %q is not supported", providerType)
	}
}
