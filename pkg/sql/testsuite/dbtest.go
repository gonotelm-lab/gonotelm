package testsuite

import (
	"crypto/rand"
	stderrors "errors"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/sql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var pgIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

const (
	EnvGonotelmDBHost   = "ENV_GONOTELM_DB_HOST"
	EnvGonotelmDBPort   = "ENV_GONOTELM_DB_PORT"
	EnvGonotelmDBUser   = "ENV_GONOTELM_DB_USER"
	EnvGonotelmDBPass   = "ENV_GONOTELM_DB_PASS"
	EnvGonotelmDBDBName = "ENV_GONOTELM_DB_DBNAME"
)

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
		missing := make([]string, 0, 5)

		host := strings.TrimSpace(os.Getenv(EnvGonotelmDBHost))
		if host == "" {
			missing = append(missing, EnvGonotelmDBHost)
		}
		portStr := strings.TrimSpace(os.Getenv(EnvGonotelmDBPort))
		if portStr == "" {
			missing = append(missing, EnvGonotelmDBPort)
		}
		user := strings.TrimSpace(os.Getenv(EnvGonotelmDBUser))
		if user == "" {
			missing = append(missing, EnvGonotelmDBUser)
		}
		pass := strings.TrimSpace(os.Getenv(EnvGonotelmDBPass))
		if pass == "" {
			missing = append(missing, EnvGonotelmDBPass)
		}
		dbName := strings.TrimSpace(os.Getenv(EnvGonotelmDBDBName))
		if dbName == "" {
			missing = append(missing, EnvGonotelmDBDBName)
		}

		if len(missing) > 0 {
			return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid env %s=%q: %w", EnvGonotelmDBPort, portStr, err)
		}

		return NewTestGormDB("pgsql", &sql.Config{
			Host:     host,
			Port:     port,
			User:     user,
			Password: pass,
			DbName:   dbName,
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

func (t *TestDb) Setup(migrationFilePath string) error {
	if t == nil {
		return fmt.Errorf("test db is nil")
	}
	switch t.driver {
	case "pgsql":
		return t.setupPgsql(migrationFilePath)
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

func (t *TestDb) setupPgsql(migrationFilePath string) error {
	if strings.TrimSpace(migrationFilePath) == "" {
		return fmt.Errorf("migration file path is empty")
	}
	if t.db != nil {
		return fmt.Errorf("test db already setup")
	}

	testDBName, err := newRandomTestDBName()
	if err != nil {
		return err
	}
	if err := createPgDatabase(&t.config, testDBName, t.logger); err != nil {
		return err
	}

	testConfig := t.config
	testConfig.DbName = testDBName
	testDB, err := sql.OpenPgSqlWithLogger(&testConfig, t.logger)
	if err != nil {
		_ = dropPgDatabase(&t.config, testDBName, t.logger)
		return fmt.Errorf("open test db failed: %w", err)
	}

	statements, err := readMigrationStatements(migrationFilePath)
	if err != nil {
		_ = closeGormDB(testDB)
		_ = dropPgDatabase(&t.config, testDBName, t.logger)
		return err
	}

	for _, statement := range statements {
		if err := testDB.Exec(statement).Error; err != nil {
			_ = closeGormDB(testDB)
			_ = dropPgDatabase(&t.config, testDBName, t.logger)
			return fmt.Errorf("execute migration statement failed: %w", err)
		}
	}

	t.testDBName = testDBName
	t.db = testDB
	return nil
}

func (t *TestDb) cleanupPgsql() error {
	var errs []error
	errs = append(errs, closeGormDB(t.db))
	t.db = nil

	if t.testDBName != "" {
		errs = append(errs, dropTestTables(&t.config, t.testDBName, t.logger))
		errs = append(errs, dropPgDatabase(&t.config, t.testDBName, t.logger))
	}
	t.testDBName = ""

	return joinErrors(errs...)
}

func createPgDatabase(config *sql.Config, dbName string, gormLogger gormlogger.Interface) error {
	quotedName, err := quotePGIdentifier(dbName)
	if err != nil {
		return err
	}

	adminDB, err := openPgAdminDB(config, gormLogger)
	if err != nil {
		return fmt.Errorf("open admin db failed: %w", err)
	}
	defer func() {
		_ = closeGormDB(adminDB)
	}()

	createDBSQL := fmt.Sprintf(`CREATE DATABASE %s`, quotedName)
	if err := adminDB.Exec(createDBSQL).Error; err != nil {
		return fmt.Errorf("create test db failed: %w", err)
	}
	return nil
}

func dropPgDatabase(config *sql.Config, dbName string, gormLogger gormlogger.Interface) error {
	quotedName, err := quotePGIdentifier(dbName)
	if err != nil {
		return err
	}

	adminDB, err := openPgAdminDB(config, gormLogger)
	if err != nil {
		return fmt.Errorf("open admin db failed: %w", err)
	}
	defer func() {
		_ = closeGormDB(adminDB)
	}()

	dropWithForceSQL := fmt.Sprintf(`DROP DATABASE IF EXISTS %s WITH (FORCE)`, quotedName)
	if err := adminDB.Exec(dropWithForceSQL).Error; err != nil {
		dropSQL := fmt.Sprintf(`DROP DATABASE IF EXISTS %s`, quotedName)
		if fallbackErr := adminDB.Exec(dropSQL).Error; fallbackErr != nil {
			return fmt.Errorf("drop test db failed, force=%v fallback=%v", err, fallbackErr)
		}
	}
	return nil
}

func dropTestTables(adminConfig *sql.Config, testDBName string, gormLogger gormlogger.Interface) error {
	testConfig := *adminConfig
	testConfig.DbName = testDBName

	testDB, err := sql.OpenPgSqlWithLogger(&testConfig, gormLogger)
	if err != nil {
		return fmt.Errorf("open test db for dropping tables failed: %w", err)
	}
	defer func() {
		_ = closeGormDB(testDB)
	}()

	if err := testDB.Exec(`DROP TABLE IF EXISTS sources`).Error; err != nil {
		return fmt.Errorf("drop table sources failed: %w", err)
	}
	if err := testDB.Exec(`DROP TABLE IF EXISTS notebooks`).Error; err != nil {
		return fmt.Errorf("drop table notebooks failed: %w", err)
	}
	return nil
}

func readMigrationStatements(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read migration file failed: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	filteredLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if trimmed == "" || strings.HasPrefix(lower, "--") {
			continue
		}
		// Skip DB-level commands from migration file, keep schema DDL only.
		if strings.HasPrefix(lower, "create database ") || strings.HasPrefix(lower, "\\c ") {
			continue
		}
		filteredLines = append(filteredLines, line)
	}

	rawStatements := strings.Split(strings.Join(filteredLines, "\n"), ";")
	statements := make([]string, 0, len(rawStatements))
	for _, raw := range rawStatements {
		statement := strings.TrimSpace(raw)
		if statement == "" {
			continue
		}
		statements = append(statements, statement)
	}
	if len(statements) == 0 {
		return nil, fmt.Errorf("no executable statements in migration file: %s", path)
	}

	return statements, nil
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
		if strings.TrimSpace(config.DbName) == "" {
			return fmt.Errorf("db name is empty")
		}
		return nil
	case "sqlite":
		if strings.TrimSpace(config.DbName) == "" {
			return fmt.Errorf("sqlite db name/path is empty")
		}
		return nil
	default:
		return fmt.Errorf("driver %s validation is not implemented yet", driver)
	}
}

func closeGormDB(db *gorm.DB) error {
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

func newRandomTestDBName() (string, error) {
	randBytes := make([]byte, 4)
	if _, err := rand.Read(randBytes); err != nil {
		return "", fmt.Errorf("read random bytes failed: %w", err)
	}

	name := fmt.Sprintf("gonotelm_test_%d_%x", time.Now().UnixNano(), randBytes)
	if len(name) > 63 {
		name = name[:63]
	}
	if !pgIdentifierPattern.MatchString(name) {
		return "", fmt.Errorf("generated invalid db name: %s", name)
	}

	return name, nil
}

func openPgAdminDB(config *sql.Config, gormLogger gormlogger.Interface) (*gorm.DB, error) {
	if config == nil {
		return nil, fmt.Errorf("db config is nil")
	}

	candidates := make([]string, 0, 3)
	candidates = append(candidates, "postgres", "template1")
	if dbName := strings.TrimSpace(config.DbName); dbName != "" {
		candidates = append(candidates, dbName)
	}

	var errs []error
	for _, dbName := range candidates {
		adminConfig := *config
		adminConfig.DbName = dbName

		db, err := sql.OpenPgSqlWithLogger(&adminConfig, gormLogger)
		if err == nil {
			return db, nil
		}
		errs = append(errs, fmt.Errorf("connect %s failed: %w", dbName, err))
	}

	return nil, joinErrors(errs...)
}

func joinErrors(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return stderrors.Join(filtered...)
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
