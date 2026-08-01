package redis

import (
	"testing"

	cacheerrors "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestChatSuggestionCacheImpl(t *testing.T) {
	chatId := "test" + uuid.NewV7().String()
	err := testChatSuggestionCache.Set(t.Context(), chatId, &schema.ChatSuggestion{
		Type:      "follow_up",
		Questions: []string{"问题1", "问题2"},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	got, err := testChatSuggestionCache.Get(t.Context(), chatId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if got.Type != "follow_up" {
		t.Fatalf("expected type follow_up, got %s", got.Type)
	}
	if len(got.Questions) != 2 {
		t.Fatalf("expected 2 questions, got %d", len(got.Questions))
	}
	if got.Questions[0] != "问题1" || got.Questions[1] != "问题2" {
		t.Fatalf("expected [问题1 问题2], got %v", got.Questions)
	}

	testChatSuggestionCache.Delete(t.Context(), chatId)
}

func TestChatSuggestionCacheImplNotFound(t *testing.T) {
	chatId := "test" + uuid.NewV7().String()
	_, err := testChatSuggestionCache.Get(t.Context(), chatId)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != cacheerrors.ErrSuggestionNotFound {
		t.Fatalf("expected %v, got %v", cacheerrors.ErrSuggestionNotFound, err)
	}
}
