package postgres18

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	pkgsql "github.com/gonotelm-lab/gonotelm/pkg/sql"
)

// Up 建库（不存在时）、打开连接并应用所有未执行的迁移。
func Up(ctx context.Context, cfg *pkgsql.Config) error {
	if cfg == nil {
		return errors.New("migrate: db config is nil")
	}

	slog.InfoContext(ctx, "ensuring database exists", "database", cfg.DBName)
	if err := pkgsql.EnsurePgDatabase(cfg, nil); err != nil {
		return fmt.Errorf("ensure database: %w", err)
	}

	slog.InfoContext(ctx, "opening database connection",
		"addr", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), "database", cfg.DBName)
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

	slog.InfoContext(ctx, "applying migrations", "database", cfg.DBName)
	return Migrate(ctx, stdDB)
}
