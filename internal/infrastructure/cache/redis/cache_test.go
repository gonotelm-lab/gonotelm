package redis

import (
	"os"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	goredis "github.com/redis/go-redis/v9"
)

var (
	testRedis                   goredis.UniversalClient
	testChatMessageContextCache cache.ChatContextMessageCache
	testChatMessageStreamCache  cache.ChatMessageStreamCache
)

func TestMain(m *testing.M) {
	addrs := os.Getenv("GONOTELM_REDIS_ADDRS")
	testRedis = goredis.NewUniversalClient(&goredis.UniversalOptions{
		Addrs:                 strings.Split(addrs, ","),
		ContextTimeoutEnabled: true,
	})

	testChatMessageContextCache = NewChatMessageContextCacheImpl(testRedis)
	testChatMessageStreamCache = NewChatMessageStreamCacheImpl(testRedis)

	m.Run()
}
