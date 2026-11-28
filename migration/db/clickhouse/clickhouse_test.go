package clickhouse

import (
	"io/fs"
	"os"
	"strconv"
	"strings"
	"testing"

	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"
	chtestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/clickhouse"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 空文件或漏写注解的文件会让 goose 在运行时才报错，这里提前拦住
func TestEmbeddedMigrations_AreGooseFormatted(t *testing.T) {
	names, err := fs.Glob(FS(), "*.sql")
	require.NoError(t, err)
	require.NotEmpty(t, names, "至少需要一个迁移文件")

	for _, name := range names {
		content, err := fs.ReadFile(FS(), name)
		require.NoError(t, err)

		body := strings.TrimSpace(string(content))
		require.NotEmpty(t, body, "%s 是空文件", name)
		assert.True(t, strings.HasPrefix(body, "-- +goose Up"),
			"%s 必须以 -- +goose Up 注解开头", name)
	}
}

func TestMigrate_RejectsNilDB(t *testing.T) {
	err := Migrate(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db is nil")
}

func TestUp_RejectsNilConfig(t *testing.T) {
	err := Up(t.Context(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db config is nil")
}

func TestUp_RejectsInvalidDBName(t *testing.T) {
	err := Up(t.Context(), &pkgsql.Config{DBName: "bad-name;"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid clickhouse identifier")
}

func TestUp_AppliesAndIsIdempotent(t *testing.T) {
	if strings.TrimSpace(os.Getenv(chtestsuite.EnvGonotelmTestClickHouseHost)) == "" {
		t.Skip("clickhouse test env is not set")
	}

	testDB, err := chtestsuite.NewTestDBFromEnv()
	require.NoError(t, err)
	require.NoError(t, testDB.Setup("unclustered/0001_init.sql"))
	t.Cleanup(func() {
		require.NoError(t, testDB.Cleanup())
	})

	port, err := strconv.Atoi(os.Getenv(chtestsuite.EnvGonotelmTestClickHousePort))
	require.NoError(t, err)

	cfg := &pkgsql.Config{
		Host:     os.Getenv(chtestsuite.EnvGonotelmTestClickHouseHost),
		Port:     port,
		User:     os.Getenv(chtestsuite.EnvGonotelmTestClickHouseUser),
		Password: os.Getenv(chtestsuite.EnvGonotelmTestClickHousePass),
		DBName:   testDB.DBName(),
	}

	require.NoError(t, Up(t.Context(), cfg))
	require.NoError(t, Up(t.Context(), cfg))
}
