package postgres

import (
	"context"
	"crypto/md5"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testIdentifier() string { return uuid.NewV4().String()[:12] }

func md5Bytes(s string) []byte {
	sum := md5.Sum([]byte(s))
	return sum[:]
}

func TestArtifactStylePreviewStore_UpsertAndGetByIdentifier(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStylePreviewStore

	identifier := testIdentifier()
	now := nowMilli()
	sum := md5Bytes("default")
	in := &schema.ArtifactStylePreview{
		StoreKey:         "sk-1",
		Identifier:       identifier,
		Md5sum:           sum,
		OriginalFilename: "default.webp",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	require.NoError(t, store.Upsert(ctx, in))

	got, err := store.GetByIdentifier(ctx, identifier)
	require.NoError(t, err)
	assert.NotZero(t, got.Id)
	assert.Equal(t, "sk-1", got.StoreKey)
	assert.Equal(t, sum, got.Md5sum)
	assert.Equal(t, "default.webp", got.OriginalFilename)
	assert.Equal(t, now, got.CreatedAt)
}

func TestArtifactStylePreviewStore_UpsertUpdatesByIdentifier(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStylePreviewStore

	identifier := testIdentifier()
	now := nowMilli()
	require.NoError(t, store.Upsert(ctx, &schema.ArtifactStylePreview{
		StoreKey:         "sk-1",
		Identifier:       identifier,
		Md5sum:           md5Bytes("a"),
		OriginalFilename: "a.webp",
		CreatedAt:        now,
		UpdatedAt:        now,
	}))

	require.NoError(t, store.Upsert(ctx, &schema.ArtifactStylePreview{
		StoreKey:         "sk-2",
		Identifier:       identifier,
		Md5sum:           md5Bytes("b"),
		OriginalFilename: "b.webp",
		CreatedAt:        now + 9999,
		UpdatedAt:        now + 1000,
	}))

	got, err := store.GetByIdentifier(ctx, identifier)
	require.NoError(t, err)
	assert.Equal(t, "sk-2", got.StoreKey)
	assert.Equal(t, md5Bytes("b"), got.Md5sum)
	assert.Equal(t, "b.webp", got.OriginalFilename)
	assert.Equal(t, now+1000, got.UpdatedAt)
	assert.Equal(t, now, got.CreatedAt)
}

func TestArtifactStylePreviewStore_GetByIdentifier_NotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStylePreviewStore

	_, err := store.GetByIdentifier(ctx, testIdentifier())
	assert.True(t, errors.Is(err, errors.ErrNoRecord))
}

func TestArtifactStylePreviewStore_ListByIdentifiers(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStylePreviewStore

	id1, id2, id3 := testIdentifier(), testIdentifier(), testIdentifier()
	now := nowMilli()
	for _, identifier := range []string{id1, id2, id3} {
		require.NoError(t, store.Upsert(ctx, &schema.ArtifactStylePreview{
			StoreKey:   "sk-" + identifier,
			Identifier: identifier,
			CreatedAt:  now,
			UpdatedAt:  now,
		}))
	}

	got, err := store.ListByIdentifiers(ctx, []string{id1, id3})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, id1, got[0].Identifier)
	assert.Equal(t, id3, got[1].Identifier)

	empty, err := store.ListByIdentifiers(ctx, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)

	missing, err := store.ListByIdentifiers(ctx, []string{testIdentifier()})
	require.NoError(t, err)
	assert.Empty(t, missing)
}

func TestArtifactStylePreviewStore_DeleteByIdentifier(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := testArtifactStylePreviewStore

	identifier := testIdentifier()
	now := nowMilli()
	require.NoError(t, store.Upsert(ctx, &schema.ArtifactStylePreview{
		StoreKey:   "sk-1",
		Identifier: identifier,
		CreatedAt:  now,
		UpdatedAt:  now,
	}))

	require.NoError(t, store.DeleteByIdentifier(ctx, identifier))

	_, err := store.GetByIdentifier(ctx, identifier)
	assert.True(t, errors.Is(err, errors.ErrNoRecord))
}
