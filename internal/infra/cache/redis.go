package cache

import (
	"context"
	"log/slog"
	"sync"

	"github.com/redis/go-redis/v9"
)

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
				slog.InfoContext(ctx, "redis connected", "addr", cfg.Addrs)
				return nil
			},
		})
	})

	return nil
}

func GetRedis() redis.UniversalClient {
	return gRedis
}
