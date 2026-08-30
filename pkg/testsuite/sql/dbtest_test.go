package testsuite

import (
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTestGormDBFromEnv_IgnoresDBNameEnv(t *testing.T) {
	t.Setenv(EnvGonotelmTestDBHost, "127.0.0.1")
	t.Setenv(EnvGonotelmTestDBPort, "5432")
	t.Setenv(EnvGonotelmTestDBUser, "postgres")
	t.Setenv(EnvGonotelmTestDBPass, "postgres")
	t.Setenv("GONOTELM_DB_NAME", "should_be_ignored")

	db, err := NewTestGormDBFromEnv("pgsql")
	require.NoError(t, err)
	require.NotNil(t, db)
	assert.Empty(t, db.config.DBName)
}

func TestNewTestGormDB_ClearsCallerDBName(t *testing.T) {
	db, err := NewTestGormDB("pgsql", &sql.Config{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
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
	assert.True(t, pgIdentifierPattern.MatchString(name), "got %q", name)
}
