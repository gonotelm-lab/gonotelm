package postgres18

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUp_RejectsNilConfig(t *testing.T) {
	err := Up(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db config is nil")
}
