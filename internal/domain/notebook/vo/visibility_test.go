package vo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeVisibility(t *testing.T) {
	assert.Equal(t, VisibilityRestricted, NormalizeVisibility(""))
	assert.Equal(t, VisibilityPublic, NormalizeVisibility(VisibilityPublic))
	assert.Equal(t, Visibility("invalid"), NormalizeVisibility(Visibility("invalid")))
	assert.True(t, VisibilityRestricted.IsValid())
	assert.True(t, VisibilityPublic.IsValid())
	assert.False(t, Visibility("invalid").IsValid())

	assert.True(t, VisibilityRestricted.IsRestricted())
	assert.False(t, VisibilityRestricted.IsPublic())
	assert.True(t, VisibilityPublic.IsPublic())
	assert.False(t, VisibilityPublic.IsRestricted())
}
