package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArtifactStore_CreateAndGetById(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStore

	id := uuid.NewV7()
	now := nowMilli()
	in := &schema.Artifact{
		Id: id, NotebookId: uuid.NewV7(), UserId: "u1",
		Kind: "mindmap", Status: "pending", FlowTaskId: "ft-1",
		Payload: []byte(`{}`), CreatedAt: now, UpdatedAt: now,
	}
	require.NoError(t, store.Create(ctx, in))

	got, err := store.GetById(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, in.UserId, got.UserId)
	assert.Equal(t, "pending", got.Status)
}

func TestArtifactStore_Upsert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStore
	id := uuid.NewV7()
	nbId := uuid.NewV7()
	now := nowMilli()

	require.NoError(t, store.Create(ctx, &schema.Artifact{
		Id: id, NotebookId: nbId, UserId: "u1",
		Kind: "report", Status: "pending", FlowTaskId: "ft-2",
		Payload: []byte(`{}`), CreatedAt: now, UpdatedAt: now,
	}))

	require.NoError(t, store.Upsert(ctx, &schema.Artifact{
		Id: id, NotebookId: nbId, UserId: "u1",
		Kind: "report", Status: "running", FlowTaskId: "ft-2",
		Payload: []byte(`{}`), CreatedAt: now, UpdatedAt: now + 1000,
	}))

	got, err := store.GetById(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "running", got.Status)
}

func TestArtifactStore_ListByStatus(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStore
	id1, id2 := uuid.NewV7(), uuid.NewV7()
	for _, id := range []uuid.UUID{id1, id2} {
		require.NoError(t, store.Create(ctx, &schema.Artifact{
			Id: id, NotebookId: uuid.NewV7(), UserId: "u3",
			Kind: "mindmap", Status: "pending", FlowTaskId: uuid.NewV7().String(),
			Payload: []byte(`{}`), CreatedAt: nowMilli(), UpdatedAt: nowMilli(),
		}))
	}
	got, err := store.ListByStatus(ctx, []string{"pending"}, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(got), 2)
}

func nowMilli() int64 { return time.Now().UnixMilli() }
