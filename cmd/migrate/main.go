// Command migrate 按 database.type / databaseOlap.type 执行 schema 迁移：
// 主库默认 postgres，OLAP 库默认 clickhouse；库不存在时会先建库，迁移可重复执行。
// 设 GONOTELM_OLAP_DB_TYPE=none 可跳过 OLAP 库。
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

	envOLAPDBType     = "GONOTELM_OLAP_DB_TYPE"
	envOLAPDBHost     = "GONOTELM_OLAP_DB_HOST"
	envOLAPDBPort     = "GONOTELM_OLAP_DB_PORT"
	envOLAPDBUser     = "GONOTELM_OLAP_DB_USER"
	envOLAPDBPassword = "GONOTELM_OLAP_DB_PASSWORD"
	envOLAPDBName     = "GONOTELM_OLAP_DB_NAME"

	// dbTypeNone 表示跳过该数据库。
	dbTypeNone = "none"
)

type target struct {
	name   string
	dbType string
	cfg    *pkgsql.Config
}

type dbEnv struct {
	host     string
	port     string
	user     string
	password string
	name     string
}

type dbDefaults struct {
	host     string
	port     int
	user     string
	password string
	name     string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	targets, err := loadTargets()
	if err != nil {
		return err
	}

	for i, t := range targets {
		printBanner(i+1, len(targets), t)
		if err := migration.Up(ctx, t.dbType, t.cfg); err != nil {
			return fmt.Errorf("%s migration: %w", t.name, err)
		}
	}

	slog.InfoContext(ctx, "migrate done", "databases", len(targets))
	return nil
}

func printBanner(index, total int, t target) {
	line := strings.Repeat("=", 72)
	fmt.Fprintf(os.Stdout, "\n%s\n  [%d/%d] %s: %s\n%s\n",
		line, index, total, strings.ToUpper(t.name), t.dbType, line)
}

func loadTargets() ([]target, error) {
	mainCfg, err := loadDBConfig(dbEnv{
		host:     envDBHost,
		port:     envDBPort,
		user:     envDBUser,
		password: envDBPassword,
		name:     envDBName,
	}, dbDefaults{
		host:     "127.0.0.1",
		port:     5432,
		user:     "postgres",
		password: "postgres",
		name:     "gonotelm",
	})
	if err != nil {
		return nil, err
	}

	targets := []target{{
		name:   "main db",
		dbType: envOr(envDBType, "postgres"),
		cfg:    mainCfg,
	}}

	olapType := envOr(envOLAPDBType, "clickhouse")
	if strings.EqualFold(olapType, dbTypeNone) {
		return targets, nil
	}

	olapCfg, err := loadDBConfig(dbEnv{
		host:     envOLAPDBHost,
		port:     envOLAPDBPort,
		user:     envOLAPDBUser,
		password: envOLAPDBPassword,
		name:     envOLAPDBName,
	}, dbDefaults{
		host:     "127.0.0.1",
		port:     9000,
		user:     "clickhouse",
		password: "clickhouse",
		name:     "gonotelm",
	})
	if err != nil {
		return nil, err
	}

	return append(targets, target{name: "olap db", dbType: olapType, cfg: olapCfg}), nil
}

func loadDBConfig(keys dbEnv, defaults dbDefaults) (*pkgsql.Config, error) {
	portStr := envOr(keys.port, strconv.Itoa(defaults.port))
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid env %s=%q: %w", keys.port, portStr, err)
	}

	return &pkgsql.Config{
		Host:     envOr(keys.host, defaults.host),
		Port:     port,
		User:     envOr(keys.user, defaults.user),
		Password: envOr(keys.password, defaults.password),
		DBName:   envOr(keys.name, defaults.name),
	}, nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
