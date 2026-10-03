package postgres18

import (
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 空文件或漏写注解的文件会让 goose 在运行时才报错，这里提前拦住。
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
