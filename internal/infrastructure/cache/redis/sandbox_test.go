package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestSandboxCacheImpl(t *testing.T) {
	userId := "test" + uuid.NewV7().String()
	notebookId := "test" + uuid.NewV7().String()
	sandboxId := "sb-" + uuid.NewV7().String()

	err := testSandboxCache.Set(t.Context(), userId, notebookId, &schema.SandboxDescription{
		Id:      sandboxId,
		Key:     schema.SandboxKey{UserId: userId, NotebookId: notebookId},
		Runtime: "test",
	}, time.Hour)
	if err != nil {
		t.Fatal(err.Error())
	}

	got, err := testSandboxCache.Get(t.Context(), userId, notebookId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if got == nil {
		t.Fatal("expected description, got nil")
	}
	if got.Id != sandboxId {
		t.Fatalf("expected id %s, got %s", sandboxId, got.Id)
	}
	if got.Runtime != "test" {
		t.Fatalf("expected runtime test, got %s", got.Runtime)
	}
	if got.Key.UserId != userId || got.Key.NotebookId != notebookId {
		t.Fatalf("unexpected key %+v", got.Key)
	}

	testSandboxCache.Delete(t.Context(), userId, notebookId)
}

func TestSandboxCacheImplNotFound(t *testing.T) {
	userId := "test" + uuid.NewV7().String()
	notebookId := "test" + uuid.NewV7().String()
	got, err := testSandboxCache.Get(t.Context(), userId, notebookId)
	if err != nil {
		t.Fatal(err.Error())
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}
