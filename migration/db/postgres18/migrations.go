// Package postgres18 提供 PostgreSQL 18 的 schema 迁移文件，并用 goose 执行迁移。
// 迁移文件与单测、cmd/migrate 共用，建库由调用方负责。
package postgres18

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var migrationFS embed.FS

// FS 返回嵌入的迁移文件，以迁移目录为根。
func FS() fs.FS {
	return migrationFS
}

// Migrate 应用所有未执行的迁移。db 必须是已连到目标库的 *sql.DB，
// 迁移进度记录在目标库的 goose 版本表里，可重复调用。
func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("migrate: db is nil")
	}

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrationFS)
	if err != nil {
		return fmt.Errorf("migrate: new goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate: apply migrations: %w", err)
	}
	if len(results) == 0 {
		return nil
	}

	versions := make([]int64, 0, len(results))
	for _, result := range results {
		versions = append(versions, result.Source.Version)
	}
	slog.InfoContext(ctx, "applied database migrations", "versions", versions)

	return nil
}
