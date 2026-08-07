package cache

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/redis/go-redis/v9/maintnotifications"
)

type ChatContextMessageCache interface {
	Append(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
	Destroy(ctx context.Context, chatId string) error
	BatchDestroy(ctx context.Context, chatIds []string) error
	ListAll(ctx context.Context, chatId string) ([]*schema.ChatContextMessage, error)
	// 从start开始获取limit条 start从0开始
	List(ctx context.Context, chatId string, start, limit int) ([]*schema.ChatContextMessage, error)
	// 获取最近的limit条
	ListRecent(ctx context.Context, chatId string, limit int) ([]*schema.ChatContextMessage, error)

	Override(ctx context.Context, chatId string, messages []*schema.ChatContextMessage) error
}

type ChatMessageStreamCache interface {
	SetTask(ctx context.Context, task *schema.ChatMessageTask) (string, error)

	GetTask(ctx context.Context, taskId string) (*schema.ChatMessageTask, error)

	DeleteTask(ctx context.Context, taskId string) error

	AppendEventStream(ctx context.Context, taskId string, event *schema.ChatMessageStreamEvent) (string, error)

	DeleteEventStream(ctx context.Context, taskId string) error

	SetEventStreamTTL(ctx context.Context, taskId string, ttl time.Duration) error

	PullEventStream(ctx context.Context, taskId string, args schema.PullEventStreamArgs) ([]*schema.ChatMessageStreamEvent, error)
}

type ChatSuggestionCache interface {
	Set(ctx context.Context, chatId string, suggestion *schema.ChatSuggestion) error
	Get(ctx context.Context, chatId string) (*schema.ChatSuggestion, error)
	Delete(ctx context.Context, chatId string) error
}

type Cache struct {
	ChatMessageContextCache ChatContextMessageCache
	ChatMessageStreamCache  ChatMessageStreamCache
	ChatSuggestionCache     ChatSuggestionCache
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
	var err error
	once.Do(func() {
		gRedis = redis.NewUniversalClient(&redis.UniversalOptions{
			Addrs:                 cfg.Addrs,
			ContextTimeoutEnabled: true,
			ClientName:            "gonotelm-redis-v9",
			Username:              cfg.Username,
			Password:              cfg.Password,
			// CLIENT MAINT_NOTIFICATIONS (Redis 8.0 feature) is not supported
			// by Redis 7.x servers; disable it to avoid handshake errors.
			MaintNotificationsConfig: &maintnotifications.Config{
				Mode: maintnotifications.ModeDisabled,
			},
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				slog.InfoContext(ctx, "created new redis connection", "addr", cfg.Addrs)
				return nil
			},
		})

		// equipped with opentelemetry
		err = redisotel.InstrumentTracing(gRedis)
	})

	return err
}

func GetRedis() redis.UniversalClient {
	return gRedis
}
