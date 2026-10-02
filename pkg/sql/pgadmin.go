package sql

import (
	"errors"
	"fmt"
	"regexp"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// pgIdentifierPattern 限制可用于 CREATE/DROP DATABASE 的库名，避免拼接 SQL 时注入。
var pgIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

// 建库/删库时用来连接维护库的候选库名。
var pgAdminDatabases = []string{"postgres", "template1"}

// IsValidPgIdentifier 判断 name 是否是安全的、无需额外转义的 PostgreSQL 标识符。
func IsValidPgIdentifier(name string) bool {
	return pgIdentifierPattern.MatchString(name)
}

// EnsurePgDatabase 在 config.DBName 不存在时创建它，已存在则直接返回。
// 建库要先连维护库，所以不会用 config.DBName 建连。
func EnsurePgDatabase(config *Config, logger gormlogger.Interface) error {
	if config == nil {
		return errors.New("db config is nil")
	}
	quotedName, err := quotePGIdentifier(config.DBName)
	if err != nil {
		return err
	}

	adminDB, err := openPgAdminDB(config, logger)
	if err != nil {
		return err
	}
	defer func() {
		_ = CloseGormDB(adminDB)
	}()

	var exists bool
	query := `SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`
	if err := adminDB.Raw(query, config.DBName).Row().Scan(&exists); err != nil {
		return fmt.Errorf("check database %q existence failed: %w", config.DBName, err)
	}
	if exists {
		return nil
	}

	if err := adminDB.Exec("CREATE DATABASE " + quotedName).Error; err != nil {
		return fmt.Errorf("create database %q failed: %w", config.DBName, err)
	}
	return nil
}

// DropPgDatabase 删除 config.DBName 并强制断开已有连接（PostgreSQL 13+ 的 WITH (FORCE)），
// 库不存在时直接返回。
func DropPgDatabase(config *Config, logger gormlogger.Interface) error {
	if config == nil {
		return errors.New("db config is nil")
	}
	quotedName, err := quotePGIdentifier(config.DBName)
	if err != nil {
		return err
	}

	adminDB, err := openPgAdminDB(config, logger)
	if err != nil {
		return err
	}
	defer func() {
		_ = CloseGormDB(adminDB)
	}()

	forceSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", quotedName)
	if err := adminDB.Exec(forceSQL).Error; err != nil {
		fallbackSQL := fmt.Sprintf("DROP DATABASE IF EXISTS %s", quotedName)
		if fallbackErr := adminDB.Exec(fallbackSQL).Error; fallbackErr != nil {
			return fmt.Errorf("drop database %q failed, force=%v fallback=%v", config.DBName, err, fallbackErr)
		}
	}
	return nil
}

// CloseGormDB 关闭 gorm 底层的 *sql.DB 连接池。
func CloseGormDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql db failed: %w", err)
	}
	return sqlDB.Close()
}

func quotePGIdentifier(identifier string) (string, error) {
	if !pgIdentifierPattern.MatchString(identifier) {
		return "", fmt.Errorf("invalid postgres identifier: %s", identifier)
	}
	return fmt.Sprintf(`"%s"`, identifier), nil
}

func openPgAdminDB(config *Config, logger gormlogger.Interface) (*gorm.DB, error) {
	errs := make([]error, 0, len(pgAdminDatabases))
	for _, dbName := range pgAdminDatabases {
		adminConfig := *config
		adminConfig.DBName = dbName

		db, err := OpenPgSqlWithLogger(&adminConfig, logger)
		if err == nil {
			return db, nil
		}
		errs = append(errs, fmt.Errorf("connect %s failed: %w", dbName, err))
	}
	return nil, errors.Join(errs...)
}
