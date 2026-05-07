package impl

import (
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	ChatMessageContextCache cache.ChatContextMessageCache
	ChatMessageTaskCache    cache.ChatMessageTaskCache
}

func NewCache(
	redis redis.UniversalClient,
) *Cache {
	return &Cache{
		ChatMessageContextCache: NewChatMessageContextCacheImpl(redis),
		ChatMessageTaskCache:    NewChatMessageTaskCacheImpl(redis),
	}
}
