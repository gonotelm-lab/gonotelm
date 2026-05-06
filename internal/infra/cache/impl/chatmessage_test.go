package impl

import (
	"os"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/redis/go-redis/v9"
)

var testRedis redis.UniversalClient

func TestMain(m *testing.M) {
	addrs := os.Getenv("ENV_GONOTELM_REDIS_ADDRS")
	testRedis = redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: strings.Split(addrs, ","),
	})

	m.Run()
}

func TestChatMessageContextCacheImpl(t *testing.T) {
	cache := NewChatMessageContextCacheImpl(testRedis)
	chatId := "test" + uuid.NewV7().String()
	err := cache.Append(t.Context(), chatId, []*schema.ChatContextMessage{
		{
			Message: []byte("{\"name\": \"ryan\"}"),
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	listed, err := cache.ListAll(t.Context(), chatId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 chat message context, got %d", len(listed))
	}
	if string(listed[0].Message) != "{\"name\": \"ryan\"}" {
		t.Fatalf("expected \"{\"name\": \"ryan\"}\", got %s", string(listed[0].Message))
	}
	cache.Destroy(t.Context(), chatId)
}

func TestChatMessageContextCacheImplOverride(t *testing.T) {
	cache := NewChatMessageContextCacheImpl(testRedis)
	chatId := "test" + uuid.NewV7().String()
	err := cache.Append(t.Context(), chatId, []*schema.ChatContextMessage{
		{
			Message: []byte("{\"name\": \"ryan\"}"),
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	err = cache.Override(t.Context(), chatId, []*schema.ChatContextMessage{
		{
			Message: []byte("{\"name\": \"assistant\"}"),
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	listed, err := cache.ListAll(t.Context(), chatId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 chat message context, got %d", len(listed))
	}
	if string(listed[0].Message) != "{\"name\": \"assistant\"}" {
		t.Fatalf("expected \"{\"name\": \"assistant\"}\", got %s", string(listed[0].Message))
	}
	cache.Destroy(t.Context(), chatId)
}
