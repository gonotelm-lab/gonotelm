package impl

import (
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
)

var (
	testRedis                   redis.UniversalClient
	testChatMessageContextCache cache.ChatContextMessageCache
	testChatMessageStreamCache  cache.ChatMessageStreamCache
)

func TestMain(m *testing.M) {
	addrs := os.Getenv("ENV_GONOTELM_REDIS_ADDRS")
	testRedis = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:                 strings.Split(addrs, ","),
		ContextTimeoutEnabled: true,
	})

	testChatMessageContextCache = NewChatMessageContextCacheImpl(testRedis)
	testChatMessageStreamCache = NewChatMessageStreamCacheImpl(testRedis)

	m.Run()
}
