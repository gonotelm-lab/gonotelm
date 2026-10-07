package redis

import (
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	redistestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/redis"
	goredis "github.com/redis/go-redis/v9"
	. "github.com/smartystreets/goconvey/convey"
)

var (
	testRedis                   goredis.UniversalClient
	testChatMessageContextCache cache.ChatContextMessageCache
	testChatMessageStreamCache  cache.ChatMessageStreamCache
	testChatSuggestionCache     cache.ChatSuggestionCache
	testSandboxCache            cache.SandboxCache
	testLoginInfoCache          cache.TransientProviderLoginInfoCache
	testUserSessionCache        cache.UserSessionCache
	testStylePreviewCache       cache.StylePreviewCache
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
	testLoginInfoCache = NewTransientProviderLoginInfoCacheImpl(testRedis)
	testUserSessionCache = NewUserSessionCacheImpl(testRedis)
	testStylePreviewCache = NewStylePreviewCacheImpl(testRedis)

	code := m.Run()

	if err := testRedisSuite.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup redis testsuite: %v\n", err)
	}
	os.Exit(code)
}

// NewCache 是 redis 缓存的装配入口，漏挂字段只会在运行期 panic，这里逐个断言非 nil。
func TestNewCache_WiresAllCaches(t *testing.T) {
	Convey("NewCache wires all caches", t, func() {
		c := NewCache(testRedis)
		So(c, ShouldNotBeNil)
		So(c.ChatMessageContextCache, ShouldNotBeNil)
		So(c.ChatMessageStreamCache, ShouldNotBeNil)
		So(c.ChatSuggestionCache, ShouldNotBeNil)
		So(c.SandboxCache, ShouldNotBeNil)
		So(c.LoginInfoCache, ShouldNotBeNil)
		So(c.UserSessionCache, ShouldNotBeNil)
		So(c.StylePreviewCache, ShouldNotBeNil)
	})
}
