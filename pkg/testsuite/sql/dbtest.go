package testsuite

import (
	"context"
	"crypto/rand"
	stdsql "database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/sql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	EnvGonotelmTestDBHost = "TEST_GONOTELM_DB_HOST"
	EnvGonotelmTestDBPort = "TEST_GONOTELM_DB_PORT"
	EnvGonotelmTestDBUser = "TEST_GONOTELM_DB_USER"
	EnvGonotelmTestDBPass = "TEST_GONOTELM_DB_PASS"

	// 避免迁移卡死时整个测试包一直挂住。
	migrationTimeout = 2 * time.Minute
)

// Migrator 在随机 test_ 库建好后、交给测试之前执行，用于把表结构迁移到最新版本。
type Migrator func(ctx context.Context, db *stdsql.DB) error

type TestDb struct {
	db         *gorm.DB
	driver     string
	config     sql.Config
	logger     gormlogger.Interface
	testDBName string
}

func NewTestGormDB(driver string, config *sql.Config) (*TestDb, error) {
	if config == nil {
		return nil, fmt.Errorf("db config is nil")
	}

	normalizedDriver, err := normalizeDriver(driver)
	if err != nil {
		return nil, err
	}

	cfg := *config
	if normalizedDriver == "pgsql" {
		// DB name is never caller-specified: Setup always creates an ephemeral test_* database.
		cfg.DBName = ""
	}
	if err := validateConfig(normalizedDriver, &cfg); err != nil {
		return nil, err
	}

	return &TestDb{
		driver: normalizedDriver,
		config: cfg,
		logger: newTestLogger(),
	}, nil
}

func NewTestGormDBFromEnv(driver string) (*TestDb, error) {
	normalizedDriver, err := normalizeDriver(driver)
	if err != nil {
		return nil, err
	}

	switch normalizedDriver {
	case "pgsql":
		missing := make([]string, 0, 4)

		host := strings.TrimSpace(os.Getenv(EnvGonotelmTestDBHost))
		if host == "" {
			missing = append(missing, EnvGonotelmTestDBHost)
		}
		portStr := strings.TrimSpace(os.Getenv(EnvGonotelmTestDBPort))
		if portStr == "" {
			missing = append(missing, EnvGonotelmTestDBPort)
		}
		user := strings.TrimSpace(os.Getenv(EnvGonotelmTestDBUser))
		if user == "" {
			missing = append(missing, EnvGonotelmTestDBUser)
		}
		pass := strings.TrimSpace(os.Getenv(EnvGonotelmTestDBPass))
		if pass == "" {
			missing = append(missing, EnvGonotelmTestDBPass)
		}

		if len(missing) > 0 {
			return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid env %s=%q: %w", EnvGonotelmTestDBPort, portStr, err)
		}

		return NewTestGormDB("pgsql", &sql.Config{
			Host:     host,
			Port:     port,
			User:     user,
			Password: pass,
		})
	default:
		return nil, fmt.Errorf("driver %s env loader is not implemented yet", normalizedDriver)
	}
}

func (t *TestDb) GetDB() *gorm.DB {
	if t == nil {
		return nil
	}
	return t.db
}

// Setup 创建随机 test_ 库并用 migrate 迁移表结构，任何一步失败都会把已建的库清理掉。
func (t *TestDb) Setup(ctx context.Context, migrate Migrator) error {
	if t == nil {
		return errors.New("test db is nil")
	}
	if migrate == nil {
		return errors.New("migrator is nil")
	}

	switch t.driver {
	case "pgsql":
		return t.setupPgsql(ctx, migrate)
	default:
		return fmt.Errorf("driver %s setup is not implemented yet", t.driver)
	}
}

func (t *TestDb) Cleanup() error {
	if t == nil {
		return nil
	}
	switch t.driver {
	case "pgsql":
		return t.cleanupPgsql()
	default:
		return fmt.Errorf("driver %s cleanup is not implemented yet", t.driver)
	}
}

func (t *TestDb) setupPgsql(ctx context.Context, migrate Migrator) error {
	if t.db != nil {
		return errors.New("test db already setup")
	}

	testDBName, err := newRandomTestDBName()
	if err != nil {
		return err
	}

	testConfig := t.config
	testConfig.DBName = testDBName
	if err := sql.EnsurePgDatabase(&testConfig, t.logger); err != nil {
		return err
	}

	testDB, err := sql.OpenPgSqlWithLogger(&testConfig, t.logger)
	if err != nil {
		_ = sql.DropPgDatabase(&testConfig, t.logger)
		return fmt.Errorf("open test db failed: %w", err)
	}

	stdDB, err := testDB.DB()
	if err != nil {
		_ = sql.CloseGormDB(testDB)
		_ = sql.DropPgDatabase(&testConfig, t.logger)
		return fmt.Errorf("get sql db failed: %w", err)
	}

	migrateCtx, cancel := context.WithTimeout(ctx, migrationTimeout)
	defer cancel()
	if err := migrate(migrateCtx, stdDB); err != nil {
		_ = sql.CloseGormDB(testDB)
		_ = sql.DropPgDatabase(&testConfig, t.logger)
		return fmt.Errorf("migrate test db failed: %w", err)
	}

	t.testDBName = testDBName
	t.db = testDB
	return nil
}

func (t *TestDb) cleanupPgsql() error {
	var errs []error
	errs = append(errs, sql.CloseGormDB(t.db))
	t.db = nil

	if t.testDBName != "" {
		dropConfig := t.config
		dropConfig.DBName = t.testDBName
		// 临时 test_ 库整体删除，不需要先 drop 表。
		errs = append(errs, sql.DropPgDatabase(&dropConfig, t.logger))
	}
	t.testDBName = ""

	return errors.Join(errs...)
}

func normalizeDriver(driver string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "pgsql", "postgres", "postgresql":
		return "pgsql", nil
	case "mysql":
		return "mysql", nil
	case "sqlite", "sqlite3":
		return "sqlite", nil
	default:
		return "", fmt.Errorf("unsupported driver: %s", driver)
	}
}

func validateConfig(driver string, config *sql.Config) error {
	if config == nil {
		return fmt.Errorf("db config is nil")
	}
	switch driver {
	case "pgsql", "mysql":
		if strings.TrimSpace(config.Host) == "" {
			return fmt.Errorf("db host is empty")
		}
		if config.Port <= 0 {
			return fmt.Errorf("db port must be positive")
		}
		if strings.TrimSpace(config.User) == "" {
			return fmt.Errorf("db user is empty")
		}
		if strings.TrimSpace(config.Password) == "" {
			return fmt.Errorf("db password is empty")
		}
		// DBName is unused for pgsql tests: Setup always creates an ephemeral test_* database.
		return nil
	case "sqlite":
		if strings.TrimSpace(config.DBName) == "" {
			return fmt.Errorf("sqlite db name/path is empty")
		}
		return nil
	default:
		return fmt.Errorf("driver %s validation is not implemented yet", driver)
	}
}

func newRandomTestDBName() (string, error) {
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("read random bytes failed: %w", err)
	}

	name := fmt.Sprintf("test_%d_%x", time.Now().UnixNano(), randBytes)
	if len(name) > 63 {
		name = name[:63]
	}
	if !sql.IsValidPgIdentifier(name) {
		return "", fmt.Errorf("generated invalid db name: %s", name)
	}

	return name, nil
}

func newTestLogger() gormlogger.Interface {
	return gormlogger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		gormlogger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  gormlogger.Info,
			IgnoreRecordNotFoundError: true,
			ParameterizedQueries:      false,
			Colorful:                  false,
		},
	)
}
