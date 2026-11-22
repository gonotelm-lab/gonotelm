// Package clickhouse 提供 ClickHouse 的 schema 迁移文件，并用 goose 执行迁移。
// 迁移文件与单测、cmd/migrate 共用；建库与连接由 Up 负责。
package clickhouse

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"regexp"
	"strconv"

	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pressly/goose/v3"
)

//go:embed unclustered/*.sql
var migrationFS embed.FS

var identifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// FS 返回嵌入的迁移文件，以迁移目录为根。
func FS() fs.FS {
	sub, err := fs.Sub(migrationFS, "unclustered")
	if err != nil {
		// embed 目录是编译期常量，不会失败。
		panic(err)
	}
	return sub
}

// Migrate 应用所有未执行的迁移。db 必须是已连到目标库的 *sql.DB，
// 迁移进度记录在目标库的 goose 版本表里，可重复调用。
func Migrate(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("migrate: db is nil")
	}

	provider, err := goose.NewProvider(goose.DialectClickHouse, db, FS())
	if err != nil {
		return fmt.Errorf("migrate: new goose provider: %w", err)
	}

	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("migrate: apply migrations: %w", err)
	}
	if len(results) == 0 {
		slog.InfoContext(ctx, "no pending database migrations")
		return nil
	}

	versions := make([]int64, 0, len(results))
	for _, result := range results {
		versions = append(versions, result.Source.Version)
	}
	slog.InfoContext(ctx, "applied database migrations", "versions", versions)

	return nil
}

// Up 建库（不存在时）、打开连接并应用所有未执行的迁移。
func Up(ctx context.Context, cfg *pkgsql.Config) error {
	if cfg == nil {
		return errors.New("migrate: db config is nil")
	}
	if !identifierPattern.MatchString(cfg.DBName) {
		return fmt.Errorf("invalid clickhouse identifier: %s", cfg.DBName)
	}

	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	slog.InfoContext(ctx, "ensuring database exists", "database", cfg.DBName)
	if err := ensureDatabase(ctx, cfg, addr); err != nil {
		return fmt.Errorf("ensure database: %w", err)
	}

	slog.InfoContext(ctx, "opening database connection", "addr", addr, "database", cfg.DBName)
	db := ch.OpenDB(&ch.Options{
		Addr: []string{addr},
		Auth: ch.Auth{
			Database: cfg.DBName,
			Username: cfg.User,
			Password: cfg.Password,
		},
	})
	defer func() {
		_ = db.Close()
	}()

	slog.InfoContext(ctx, "applying migrations", "database", cfg.DBName)
	return Migrate(ctx, db)
}

// ensureDatabase 连到 default 库，按需创建目标库。
func ensureDatabase(ctx context.Context, cfg *pkgsql.Config, addr string) error {
	admin := ch.OpenDB(&ch.Options{
		Addr: []string{addr},
		Auth: ch.Auth{
			Database: "default",
			Username: cfg.User,
			Password: cfg.Password,
		},
	})
	defer func() {
		_ = admin.Close()
	}()

	if _, err := admin.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+cfg.DBName); err != nil {
		return fmt.Errorf("create database %q: %w", cfg.DBName, err)
	}
	return nil
}
