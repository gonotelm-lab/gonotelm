package impl

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestChatMessageContextCacheImpl(t *testing.T) {
	chatId := "test" + uuid.NewV7().String()
	err := testChatMessageContextCache.Append(t.Context(), chatId, []*schema.ChatContextMessage{
		{
			Message: []byte("{\"name\": \"ryan\"}"),
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	listed, err := testChatMessageContextCache.ListAll(t.Context(), chatId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 chat message context, got %d", len(listed))
	}
	if string(listed[0].Message) != "{\"name\": \"ryan\"}" {
		t.Fatalf("expected \"{\"name\": \"ryan\"}\", got %s", string(listed[0].Message))
	}
	testChatMessageContextCache.Destroy(t.Context(), chatId)
}

func TestChatMessageContextCacheImplOverride(t *testing.T) {
	chatId := "test" + uuid.NewV7().String()
	err := testChatMessageContextCache.Append(t.Context(), chatId, []*schema.ChatContextMessage{
		{
			Message: []byte("{\"name\": \"ryan\"}"),
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	err = testChatMessageContextCache.Override(t.Context(), chatId, []*schema.ChatContextMessage{
		{
			Message: []byte("{\"name\": \"assistant\"}"),
		},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	listed, err := testChatMessageContextCache.ListAll(t.Context(), chatId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 chat message context, got %d", len(listed))
	}
	if string(listed[0].Message) != "{\"name\": \"assistant\"}" {
		t.Fatalf("expected \"{\"name\": \"assistant\"}\", got %s", string(listed[0].Message))
	}
	testChatMessageContextCache.Destroy(t.Context(), chatId)
}
