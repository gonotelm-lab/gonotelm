package chat

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/deepseek"
	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

func New(
	ctx context.Context,
	cfg *Config,
) (model.ToolCallingChatModel, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config must not be nil")
	}

	switch cfg.Type {
	case DeepSeek:
		return deepseek.NewChatModel(ctx, cfg.DeepSeek.ToEino())
	case Openai:
		return openai.NewChatModel(ctx, cfg.Openai.ToEino())
	default:
		return nil, fmt.Errorf("model type %q is not supported", cfg.Type)
	}
}
