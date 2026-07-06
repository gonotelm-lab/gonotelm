package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/redis/go-redis/v9"
)

type ChatContextMessageCache interface {
	Append(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
	Destroy(ctx context.Context, chatId string) error
	BatchDestroy(ctx context.Context, chatIds []string) error
	ListAll(ctx context.Context, chatId string) ([]*schema.ChatContextMessage, error)

	Override(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
}

var (
	ErrTaskNotFound = errors.New("task not found")
	ErrStreamNoData = errors.New("stream no data")
)

type ChatMessageStreamCache interface {
	SetTask(ctx context.Context, task *schema.ChatMessageTask) (string, error)

	GetTask(ctx context.Context, taskId string) (*schema.ChatMessageTask, error)

	DeleteTask(ctx context.Context, taskId string) error

	AppendEventStream(ctx context.Context, taskId string, event *schema.ChatMessageStreamEvent) (string, error)

	DeleteEventStream(ctx context.Context, taskId string) error

	SetEventStreamTTL(ctx context.Context, taskId string, ttl time.Duration) error

	PullEventStream(ctx context.Context, taskId string, args schema.PullEventStreamArgs) ([]*schema.ChatMessageStreamEvent, error)
}

type Cache struct {
	ChatMessageContextCache ChatContextMessageCache
	ChatMessageStreamCache  ChatMessageStreamCache
}

var (
	once   sync.Once
	gRedis redis.UniversalClient
)

type RedisCacheConfig struct {
	Addrs    []string `toml:"addrs"`
	Username string   `toml:"username"`
	Password string   `toml:"password"`
}

func Init(cfg *RedisCacheConfig) error {
	once.Do(func() {
		gRedis = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:                 cfg.Addrs,
			ContextTimeoutEnabled: true,
			ClientName:            "gonotelm-redis-v9",
			Username:              cfg.Username,
			Password:              cfg.Password,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				slog.InfoContext(ctx, "created new redis connection", "addr", cfg.Addrs)
				return nil
			},
		})
	})

	return nil
}

func GetRedis() redis.UniversalClient {
	return gRedis
}
