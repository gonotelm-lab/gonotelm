package mapper

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStylePreviewRoundTrip(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/default.webp", true)
	require.NoError(t, err)
	p := artifactentity.NewStylePreview("default", key)
	p.UpdateAsset(key, []byte("sum"), "default.webp")

	sch := StylePreviewToSchema(p)
	assert.Equal(t, "default", sch.Identifier)
	assert.Equal(t, key.Encode(), sch.StoreKey)
	assert.Equal(t, "default.webp", sch.OriginalFilename)

	back, err := StylePreviewFromSchema(sch)
	require.NoError(t, err)
	assert.Equal(t, p.Identifier, back.Identifier)
	assert.Equal(t, p.StoreKey, back.StoreKey)
	assert.Equal(t, p.Md5sum, back.Md5sum)
	assert.Equal(t, p.OriginalFilename, back.OriginalFilename)
	assert.Equal(t, p.CreateTime.Value(), back.CreateTime.Value())
	assert.Equal(t, p.UpdateTime.Value(), back.UpdateTime.Value())
}

func TestStylePreviewFromSchema_EmptyStoreKey(t *testing.T) {
	back, err := StylePreviewFromSchema(&schema.ArtifactStylePreview{Identifier: "default"})
	require.NoError(t, err)
	assert.Equal(t, valobj.StoreKey{}, back.StoreKey)
}

func TestStylePreviewCacheSchemaRoundTrip(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/default.webp", true)
	require.NoError(t, err)
	p := artifactentity.NewStylePreview("default", key)
	p.UpdateAsset(key, []byte("sum"), "default.webp")

	sch := StylePreviewToCacheSchema(p)
	assert.Equal(t, "default", sch.Identifier)
	assert.Equal(t, key.Encode(), sch.StoreKey)
	assert.Equal(t, "default.webp", sch.OriginalFilename)

	back, err := StylePreviewFromCacheSchema(sch)
	require.NoError(t, err)
	assert.Equal(t, p.Identifier, back.Identifier)
	assert.Equal(t, p.StoreKey, back.StoreKey)
	assert.Equal(t, p.Md5sum, back.Md5sum)
	assert.Equal(t, p.OriginalFilename, back.OriginalFilename)
	assert.Equal(t, p.CreateTime.Value(), back.CreateTime.Value())
	assert.Equal(t, p.UpdateTime.Value(), back.UpdateTime.Value())
}

func TestStylePreviewFromCacheSchema_EmptyStoreKey(t *testing.T) {
	back, err := StylePreviewFromCacheSchema(&cacheschema.StylePreview{Identifier: "default"})
	require.NoError(t, err)
	assert.Equal(t, valobj.StoreKey{}, back.StoreKey)
}
