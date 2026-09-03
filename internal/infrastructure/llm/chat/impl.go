package chat

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino-ext/components/model/qwen"
	"github.com/cloudwego/eino/components/model"
	"github.com/gonotelm-lab/gonotelm/pkg/eino-ext/model/agnes"
)

func newChatModel(
	ctx context.Context,
	providerType Provider,
	cfg *ProviderConfig,
	recorder Recorder,
) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}

	var (
		tcm            model.ToolCallingChatModel
		maxConcurrency int
		err            error
	)

	switch providerType {
	case ProviderDeepSeek:
		tcm, err = openai.NewChatModel(ctx, cfg.DeepSeek.ToOpenaiEino()) // deepseek package does not support image input so we use openai package
		maxConcurrency = cfg.DeepSeek.MaxConcurrency
	case ProviderOpenAI:
		tcm, err = openai.NewChatModel(ctx, cfg.OpenAI.ToEino())
		maxConcurrency = cfg.OpenAI.MaxConcurrency
	case ProviderQwen:
		tcm, err = qwen.NewChatModel(ctx, cfg.Qwen.ToEino())
		maxConcurrency = cfg.Qwen.MaxConcurrency
	case ProviderAgnes:
		tcm, err = agnes.NewChatModel(ctx, cfg.Agnes.ToEino())
		maxConcurrency = cfg.Agnes.MaxConcurrency
	default:
		return nil, fmt.Errorf("model type %q is not supported", providerType)
	}
	if err != nil {
		return nil, err
	}

	return newWrappedChatModel(ctx, tcm, providerType, maxConcurrency, recorder), nil
}
