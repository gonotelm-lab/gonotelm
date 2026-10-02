package redis

import (
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestUserSessionCacheImpl(t *testing.T) {
	ctx := t.Context()
	id := "sess" + uuid.NewV7().String()
	userId := "user" + uuid.NewV7().String()

	session := &schema.UserSession{
		UserId:    userId,
		CreatedAt: time.Now().UnixMilli(),
		ExpireAt:  time.Now().Add(time.Hour).UnixMilli(),
		Device:    "web",
	}

	if err := testUserSessionCache.Set(ctx, id, session, time.Hour); err != nil {
		t.Fatal(err)
	}

	got, err := testUserSessionCache.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserId != userId || got.Device != "web" {
		t.Fatalf("unexpected session: %+v", got)
	}

	if err := testUserSessionCache.DeleteByUserId(ctx, userId); err != nil {
		t.Fatal(err)
	}
	if _, err := testUserSessionCache.Get(ctx, id); !errors.Is(err, errors.ErrNoRecord) {
		t.Fatalf("expected ErrNoRecord, got %v", err)
	}
}

func TestUserSessionCacheImplDelete(t *testing.T) {
	ctx := t.Context()
	id := "sess" + uuid.NewV7().String()
	userId := "user" + uuid.NewV7().String()

	session := &schema.UserSession{UserId: userId, CreatedAt: time.Now().UnixMilli(), ExpireAt: time.Now().Add(time.Hour).UnixMilli()}
	if err := testUserSessionCache.Set(ctx, id, session, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := testUserSessionCache.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := testUserSessionCache.Get(ctx, id); !errors.Is(err, errors.ErrNoRecord) {
		t.Fatalf("expected ErrNoRecord, got %v", err)
	}
}

func TestUserSessionCacheImplNotFound(t *testing.T) {
	_, err := testUserSessionCache.Get(t.Context(), "missing"+uuid.NewV7().String())
	if !errors.Is(err, errors.ErrNoRecord) {
		t.Fatalf("expected ErrNoRecord, got %v", err)
	}
}
