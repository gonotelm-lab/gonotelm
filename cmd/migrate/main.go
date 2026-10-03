// Command migrate 用 goose 把 PostgreSQL schema 迁移到最新版本，迁移文件与单测共用
// migration/db/postgres18；库不存在时会先建库。
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/gonotelm-lab/gonotelm/migration/db/postgres18"
	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

const (
	envDBHost     = "GONOTELM_DB_HOST"
	envDBPort     = "GONOTELM_DB_PORT"
	envDBUser     = "GONOTELM_DB_USER"
	envDBPassword = "GONOTELM_DB_PASSWORD"
	envDBName     = "GONOTELM_DB_NAME"

	// 用于识别 goose 之前用 psql 建的旧库：旧库 schema 等价于迁移 0001。
	legacyBaselineVersion int64 = 1
	legacySchemaMarker          = "notebooks"
)

func main() {
	baseline := flag.Bool("baseline", false,
		"mark a pre-goose schema as migrated (needed once for databases created before goose)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *baseline); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, baseline bool) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	if err := pkgsql.EnsurePgDatabase(cfg, nil); err != nil {
		return err
	}

	gormDB, err := pkgsql.OpenPgSql(cfg)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		_ = pkgsql.CloseGormDB(gormDB)
	}()

	stdDB, err := gormDB.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}

	if baseline {
		if err := baselineLegacySchema(ctx, stdDB); err != nil {
			return err
		}
	}

	slog.InfoContext(ctx, "running database migrations", "database", cfg.DBName)
	return postgres18.Migrate(ctx, stdDB)
}

func loadConfig() (*pkgsql.Config, error) {
	portStr := envOr(envDBPort, "5432")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid env %s=%q: %w", envDBPort, portStr, err)
	}

	return &pkgsql.Config{
		Host:     envOr(envDBHost, "127.0.0.1"),
		Port:     port,
		User:     envOr(envDBUser, "postgres"),
		Password: envOr(envDBPassword, "postgres"),
		DBName:   envOr(envDBName, "gonotelm"),
	}, nil
}

// baselineLegacySchema 把 goose 之前用 psql 建的旧库标记为已应用迁移 0001，避免重复建表。
func baselineLegacySchema(ctx context.Context, db *sql.DB) error {
	hasVersionTable, err := pgRelationExists(ctx, db, goose.DefaultTablename)
	if err != nil {
		return err
	}
	if hasVersionTable {
		return nil
	}

	hasLegacySchema, err := pgRelationExists(ctx, db, legacySchemaMarker)
	if err != nil {
		return err
	}
	if !hasLegacySchema {
		return nil
	}

	store, err := database.NewStore(database.DialectPostgres, goose.DefaultTablename)
	if err != nil {
		return fmt.Errorf("create goose store: %w", err)
	}
	if err := store.CreateVersionTable(ctx, db); err != nil {
		return fmt.Errorf("create goose version table: %w", err)
	}
	if err := store.Insert(ctx, db, database.InsertRequest{Version: legacyBaselineVersion}); err != nil {
		return fmt.Errorf("baseline migration %d: %w", legacyBaselineVersion, err)
	}

	slog.WarnContext(ctx, "adopted pre-goose schema",
		"table", legacySchemaMarker,
		"version", legacyBaselineVersion)

	return nil
}

func pgRelationExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&exists); err != nil {
		return false, fmt.Errorf("check relation %q: %w", name, err)
	}
	return exists, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
