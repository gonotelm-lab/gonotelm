package redis

import (
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	goredis "github.com/redis/go-redis/v9"
)

func NewCache(
	rdb goredis.UniversalClient,
) *cache.Cache {
	return &cache.Cache{
		ChatMessageContextCache: NewChatMessageContextCacheImpl(rdb),
		ChatMessageStreamCache:  NewChatMessageStreamCacheImpl(rdb),
	}
}
