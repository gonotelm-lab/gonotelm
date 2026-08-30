package testsuite

import (
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestDBFromEnv_RequiresClickHouseEnvs(t *testing.T) {
	t.Setenv(EnvGonotelmTestClickHouseHost, "127.0.0.1")
	t.Setenv(EnvGonotelmTestClickHousePort, "9000")
	t.Setenv(EnvGonotelmTestClickHouseUser, "default")
	t.Setenv(EnvGonotelmTestClickHousePass, "clickhouse")

	db, err := NewTestDBFromEnv()
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Empty(t, db.config.DBName)
}

func TestNewTestDB_ClearsCallerDBName(t *testing.T) {
	db, err := NewTestDB(&sql.Config{
		Host:     "127.0.0.1",
		Port:     9000,
		User:     "default",
		Password: "clickhouse",
		DBName:   "caller_provided",
	})
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Empty(t, db.config.DBName)
}

func TestNewRandomTestDBName_StartsWithTestPrefix(t *testing.T) {
	name, err := newRandomTestDBName()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(name, "test_"), "got %q", name)
	assert.True(t, chIdentifierPattern.MatchString(name), "got %q", name)
}
