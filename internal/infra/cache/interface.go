package cache

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
)

// 会话上下文存储 包含大模型返回的工具调用 工具调用结果等
type ChatContextMessageCache interface {
	Append(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
	Destroy(ctx context.Context, chatId string) error
	ListAll(ctx context.Context, chatId string) ([]*schema.ChatContextMessage, error)

	// del + set all messages
	Override(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
}
