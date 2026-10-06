package entity

import (
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStylePreview(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/cute.webp", true)
	require.NoError(t, err)

	p := NewStylePreview("cute", key)
	assert.Equal(t, "cute", p.Identifier)
	assert.Equal(t, key, p.StoreKey)
	assert.NotZero(t, p.CreateTime.Value())
	assert.Equal(t, p.CreateTime.Value(), p.UpdateTime.Value())
}

func TestStylePreview_UpdateAsset(t *testing.T) {
	key, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/cute.webp", true)
	require.NoError(t, err)
	p := NewStylePreview("cute", key)

	next, err := valobj.NewStoreKey("gonotelm", "artifact-preview/slides/cute-v2.webp", true)
	require.NoError(t, err)
	p.UpdateAsset(next, []byte("sum"), "cute-v2.webp")

	assert.Equal(t, next, p.StoreKey)
	assert.Equal(t, []byte("sum"), p.Md5sum)
	assert.Equal(t, "cute-v2.webp", p.OriginalFilename)
}
