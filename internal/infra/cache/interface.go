package cache

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// 会话上下文存储 包含大模型返回的工具调用 工具调用结果等
type ChatContextMessageCache interface {
	Append(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
	Destroy(ctx context.Context, chatId string) error
	ListAll(ctx context.Context, chatId string) ([]*schema.ChatContextMessage, error)

	// del + set all messages
	Override(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
}

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrStreamNoData = errors.New("stream no data")
)

// 会话流式输出任务缓存 + 流式输出事件缓存
type ChatMessageStreamCache interface {
	// 返回任务id
	SetTask(ctx context.Context, task *schema.ChatMessageTask) (string, error)

	// 获取流式输出任务
	GetTask(ctx context.Context, taskId string) (*schema.ChatMessageTask, error)

	// 删除任务
	DeleteTask(ctx context.Context, taskId string) error

	// 追加事件
	AppendEventStream(ctx context.Context, taskId string, event *schema.ChatMessageStreamEvent) (string, error)

	// 删除事件流
	DeleteEventStream(ctx context.Context, taskId string) error

	// 设置事件流过期时间
	SetEventStreamTTL(ctx context.Context, taskId string, ttl time.Duration) error

	// 阻塞获取时间流
	PullEventStream(ctx context.Context, taskId string, args schema.PullEventStreamArgs) ([]*schema.ChatMessageStreamEvent, error)
}
