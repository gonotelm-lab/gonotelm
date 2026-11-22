package migration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupported_ContainsBuiltinEngines(t *testing.T) {
	assert.Contains(t, Supported(), "postgres")
	assert.Contains(t, Supported(), "clickhouse")
}

func TestLookup_IsCaseInsensitive(t *testing.T) {
	fn, err := lookup("  PostgreSQL ")
	require.NoError(t, err)
	assert.NotNil(t, fn)
}

func TestUp_RejectsUnknownType(t *testing.T) {
	err := Up(context.Background(), "sqlite", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `unsupported database type "sqlite"`)
	assert.Contains(t, err.Error(), "postgres")
}
