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

// 会话流式输出任务缓存 + 流式输出事件缓存
type ChatMessageTaskCache interface {
	// 返回任务id
	CreateTask(ctx context.Context, task *schema.ChatMessageTask) (string, error)

	// 获取流式输出任务
	GetTask(ctx context.Context, taskId string) (*schema.ChatMessageTask, error)

	DeleteTask(ctx context.Context, taskId string) error

	AppendEvent(ctx context.Context, taskId string, event *schema.ChatMessageTaskEvent) (string, error)

	DeleteEventStream(ctx context.Context, taskId string) error
}
