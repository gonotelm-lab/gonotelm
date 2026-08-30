package redis

import (
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	redistestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/redis"
	goredis "github.com/redis/go-redis/v9"
)

var (
	testRedis                   goredis.UniversalClient
	testChatMessageContextCache cache.ChatContextMessageCache
	testChatMessageStreamCache  cache.ChatMessageStreamCache
	testChatSuggestionCache     cache.ChatSuggestionCache
	testSandboxCache            cache.SandboxCache
)

func TestMain(m *testing.M) {
	testRedisSuite, err := redistestsuite.NewTestRedisFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "init redis testsuite: %v\n", err)
		os.Exit(1)
	}
	if err := testRedisSuite.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "setup redis testsuite: %v\n", err)
		os.Exit(1)
	}

	testRedis = testRedisSuite.GetClient()
	testChatMessageContextCache = NewChatMessageContextCacheImpl(testRedis)
	testChatMessageStreamCache = NewChatMessageStreamCacheImpl(testRedis)
	testChatSuggestionCache = NewChatSuggestionCacheImpl(testRedis)
	testSandboxCache = NewSandboxCacheImpl(testRedis)

	code := m.Run()

	if err := testRedisSuite.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup redis testsuite: %v\n", err)
	}
	os.Exit(code)
}
