package testsuite

import (
	"context"
	"crypto/rand"
	stderrors "errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

var chIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]{0,62}$`)

const (
	EnvGonotelmTestClickHouseHost = "TEST_GONOTELM_CLICKHOUSE_HOST"
	EnvGonotelmTestClickHousePort = "TEST_GONOTELM_CLICKHOUSE_PORT"
	EnvGonotelmTestClickHouseUser = "TEST_GONOTELM_CLICKHOUSE_USER"
	EnvGonotelmTestClickHousePass = "TEST_GONOTELM_CLICKHOUSE_PASS"
)

// TestDb manages an ephemeral ClickHouse database for integration tests.
// Database name is never caller-specified: Setup always creates a random test_* database.
type TestDb struct {
	conn       ch.Conn
	config     sql.Config
	testDBName string
}

func NewTestDB(config *sql.Config) (*TestDb, error) {
	if config == nil {
		return nil, fmt.Errorf("db config is nil")
	}

	cfg := *config
	cfg.DBName = ""
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	return &TestDb{config: cfg}, nil
}

func NewTestDBFromEnv() (*TestDb, error) {
	missing := make([]string, 0, 4)

	host := strings.TrimSpace(os.Getenv(EnvGonotelmTestClickHouseHost))
	if host == "" {
		missing = append(missing, EnvGonotelmTestClickHouseHost)
	}
	portStr := strings.TrimSpace(os.Getenv(EnvGonotelmTestClickHousePort))
	if portStr == "" {
		missing = append(missing, EnvGonotelmTestClickHousePort)
	}
	user := strings.TrimSpace(os.Getenv(EnvGonotelmTestClickHouseUser))
	if user == "" {
		missing = append(missing, EnvGonotelmTestClickHouseUser)
	}
	pass := strings.TrimSpace(os.Getenv(EnvGonotelmTestClickHousePass))
	if pass == "" {
		missing = append(missing, EnvGonotelmTestClickHousePass)
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: %s", strings.Join(missing, ", "))
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return nil, fmt.Errorf("invalid env %s=%q: %w", EnvGonotelmTestClickHousePort, portStr, err)
	}

	return NewTestDB(&sql.Config{
		Host:     host,
		Port:     port,
		User:     user,
		Password: pass,
	})
}

func (t *TestDb) GetConn() ch.Conn {
	if t == nil {
		return nil
	}
	return t.conn
}

func (t *TestDb) DBName() string {
	if t == nil {
		return ""
	}
	return t.testDBName
}

// OpenConn opens a new connection to the ephemeral test database.
func (t *TestDb) OpenConn() (ch.Conn, error) {
	if t == nil {
		return nil, fmt.Errorf("test db is nil")
	}
	if t.testDBName == "" {
		return nil, fmt.Errorf("test db is not setup")
	}
	return openConn(&t.config, t.testDBName)
}

func (t *TestDb) Setup(migrationFilePath string) error {
	if t == nil {
		return fmt.Errorf("test db is nil")
	}
	if strings.TrimSpace(migrationFilePath) == "" {
		return fmt.Errorf("migration file path is empty")
	}
	if t.conn != nil {
		return fmt.Errorf("test db already setup")
	}

	testDBName, err := newRandomTestDBName()
	if err != nil {
		return err
	}
	if err := createDatabase(&t.config, testDBName); err != nil {
		return err
	}

	conn, err := openConn(&t.config, testDBName)
	if err != nil {
		_ = dropDatabase(&t.config, testDBName)
		return fmt.Errorf("open test db failed: %w", err)
	}

	statements, err := readMigrationStatements(migrationFilePath)
	if err != nil {
		_ = conn.Close()
		_ = dropDatabase(&t.config, testDBName)
		return err
	}

	ctx := context.Background()
	for _, statement := range statements {
		if err := conn.Exec(ctx, statement); err != nil {
			_ = conn.Close()
			_ = dropDatabase(&t.config, testDBName)
			return fmt.Errorf("execute migration statement failed: %w", err)
		}
	}

	t.testDBName = testDBName
	t.conn = conn
	return nil
}

func (t *TestDb) Cleanup() error {
	if t == nil {
		return nil
	}

	var errs []error
	if t.conn != nil {
		errs = append(errs, t.conn.Close())
		t.conn = nil
	}
	if t.testDBName != "" {
		errs = append(errs, dropDatabase(&t.config, t.testDBName))
		t.testDBName = ""
	}
	return joinErrors(errs...)
}

func createDatabase(config *sql.Config, dbName string) error {
	if err := validateIdentifier(dbName); err != nil {
		return err
	}

	admin, err := openConn(config, "default")
	if err != nil {
		return fmt.Errorf("open admin db failed: %w", err)
	}
	defer func() { _ = admin.Close() }()

	if err := admin.Exec(context.Background(), "CREATE DATABASE "+dbName); err != nil {
		return fmt.Errorf("create test db failed: %w", err)
	}
	return nil
}

func dropDatabase(config *sql.Config, dbName string) error {
	if err := validateIdentifier(dbName); err != nil {
		return err
	}

	admin, err := openConn(config, "default")
	if err != nil {
		return fmt.Errorf("open admin db failed: %w", err)
	}
	defer func() { _ = admin.Close() }()

	if err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+dbName+" SYNC"); err != nil {
		return fmt.Errorf("drop test db failed: %w", err)
	}
	return nil
}

func openConn(config *sql.Config, database string) (ch.Conn, error) {
	if config == nil {
		return nil, fmt.Errorf("db config is nil")
	}
	return ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Host, config.Port)},
		Auth: ch.Auth{
			Database: database,
			Username: config.User,
			Password: config.Password,
		},
	})
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
		// Skip DB-level commands; Setup creates an ephemeral test_* database instead.
		if strings.HasPrefix(lower, "create database ") {
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

func validateConfig(config *sql.Config) error {
	if config == nil {
		return fmt.Errorf("db config is nil")
	}
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
	return nil
}

func validateIdentifier(identifier string) error {
	if !chIdentifierPattern.MatchString(identifier) {
		return fmt.Errorf("invalid clickhouse identifier: %s", identifier)
	}
	return nil
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
	if err := validateIdentifier(name); err != nil {
		return "", fmt.Errorf("generated invalid db name: %s", name)
	}
	return name, nil
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
