package redis

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
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

func TestChatMessageContextCacheImplList(t *testing.T) {
	chatId := "test" + uuid.NewV7().String()
	defer testChatMessageContextCache.Destroy(t.Context(), chatId)

	messages := make([]*schema.ChatContextMessage, 0, 5)
	for idx := range 5 {
		messages = append(messages, &schema.ChatContextMessage{
			Message: []byte(string(rune('a' + idx))),
		})
	}
	if err := testChatMessageContextCache.Append(t.Context(), chatId, messages); err != nil {
		t.Fatal(err.Error())
	}

	listed, err := testChatMessageContextCache.List(t.Context(), chatId, 1, 2)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(listed) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(listed))
	}
	if string(listed[0].Message) != "b" || string(listed[1].Message) != "c" {
		t.Fatalf("expected [b c], got %v", []string{string(listed[0].Message), string(listed[1].Message)})
	}

	recent, err := testChatMessageContextCache.ListRecent(t.Context(), chatId, 2)
	if err != nil {
		t.Fatal(err.Error())
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 recent chat messages, got %d", len(recent))
	}
	if string(recent[0].Message) != "d" || string(recent[1].Message) != "e" {
		t.Fatalf("expected [d e], got %v", []string{string(recent[0].Message), string(recent[1].Message)})
	}
}
