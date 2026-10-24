package postgres18

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"

	sqltestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/sql"
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

func TestMigrate_AppliesAndIsIdempotent(t *testing.T) {
	ctx := t.Context()

	testDB, err := sqltestsuite.NewTestGormDBFromEnv("pgsql")
	require.NoError(t, err)
	require.NoError(t, testDB.Setup(ctx, func(context.Context, *sql.DB) error { return nil }))
	t.Cleanup(func() {
		require.NoError(t, testDB.Cleanup())
	})

	stdDB, err := testDB.GetDB().DB()
	require.NoError(t, err)

	require.NoError(t, Migrate(ctx, stdDB))
	require.NoError(t, Migrate(ctx, stdDB))
}
