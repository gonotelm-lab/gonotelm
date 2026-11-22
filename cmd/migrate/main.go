// Command migrate 按 database.type（GONOTELM_DB_TYPE，默认 postgres）执行 schema 迁移。
// 库不存在时会先建库，迁移可重复执行。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/gonotelm-lab/gonotelm/migration"
	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"
)

const (
	envDBType     = "GONOTELM_DB_TYPE"
	envDBHost     = "GONOTELM_DB_HOST"
	envDBPort     = "GONOTELM_DB_PORT"
	envDBUser     = "GONOTELM_DB_USER"
	envDBPassword = "GONOTELM_DB_PASSWORD"
	envDBName     = "GONOTELM_DB_NAME"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	dbType := envOr(envDBType, "postgres")
	slog.InfoContext(ctx, "migrate start",
		"type", dbType,
		"addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		"database", cfg.DBName)

	if err := migration.Up(ctx, dbType, cfg); err != nil {
		return err
	}

	slog.InfoContext(ctx, "migrate done", "type", dbType, "database", cfg.DBName)
	return nil
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

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
