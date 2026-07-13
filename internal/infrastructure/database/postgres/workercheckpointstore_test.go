package postgres

import (
	"context"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerCheckpointStore_CreateAndGetByArtifactId(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testWorkerCheckpointStore

	now := nowMilli()
	aId := uuid.NewV7()
	cp := &schema.WorkerCheckpoint{
		ArtifactId: aId,
		Field1:     []byte("hello"),
		Field2:     []byte("world"),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	require.NoError(t, store.Create(ctx, cp))

	got, err := store.GetByArtifactId(ctx, aId)
	require.NoError(t, err)
	assert.Equal(t, aId, got.ArtifactId)
	assert.Equal(t, []byte("hello"), got.Field1)
	assert.Equal(t, []byte("world"), got.Field2)
	assert.Nil(t, got.Field3)
	assert.Nil(t, got.Field4)
	assert.Nil(t, got.Field5)
	assert.Nil(t, got.Field6)
	assert.Nil(t, got.Field7)
	assert.Nil(t, got.Field8)
}

func TestWorkerCheckpointStore_GetByArtifactId_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testWorkerCheckpointStore

	_, err := store.GetByArtifactId(ctx, uuid.NewV7())
	assert.True(t, errors.Is(err, errors.ErrNoRecord))
}

func TestWorkerCheckpointStore_Update(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testWorkerCheckpointStore

	aId := uuid.NewV7()
	now := nowMilli()
	cp := &schema.WorkerCheckpoint{
		ArtifactId: aId,
		Field1:     []byte("initial1"),
		Field2:     []byte("initial2"),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	require.NoError(t, store.Create(ctx, cp))

	require.NoError(t, store.Update(ctx, &schema.WorkerCheckpoint{
		ArtifactId: aId,
		Field1:     []byte("updated1"),
	}))

	got, err := store.GetByArtifactId(ctx, aId)
	require.NoError(t, err)
	assert.Equal(t, []byte("updated1"), got.Field1)
	assert.Equal(t, []byte("initial2"), got.Field2)
}

func TestWorkerCheckpointStore_DeleteByArtifactId(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testWorkerCheckpointStore

	aId := uuid.NewV7()
	now := nowMilli()
	cp := &schema.WorkerCheckpoint{
		ArtifactId: aId,
		Field1:     []byte("y"),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	require.NoError(t, store.Create(ctx, cp))

	require.NoError(t, store.DeleteByArtifactId(ctx, aId))

	_, err := store.GetByArtifactId(ctx, aId)
	assert.True(t, errors.Is(err, errors.ErrNoRecord))
}
