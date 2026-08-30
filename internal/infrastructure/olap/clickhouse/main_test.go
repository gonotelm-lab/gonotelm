package clickhouse

import (
	"context"
	"fmt"
	"os"
	"testing"

	chtestsuite "github.com/gonotelm-lab/gonotelm/pkg/testsuite/clickhouse"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

var (
	testDB     *chtestsuite.TestDb
	testDriver ch.Conn
)

func TestMain(m *testing.M) {
	const migrationFilePath = "../../../../migration/db/clickhouse/unclustered/0001.up.sql"

	var err error
	testDB, err = chtestsuite.NewTestDBFromEnv()
	if err != nil {
		fmt.Fprintf(os.Stderr, "init clickhouse testsuite: %v\n", err)
		os.Exit(1)
	}
	if err := testDB.Setup(migrationFilePath); err != nil {
		fmt.Fprintf(os.Stderr, "setup clickhouse test db: %v\n", err)
		os.Exit(1)
	}
	testDriver = testDB.GetConn()

	ctx := context.Background()
	if err := testDriver.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping clickhouse: %v\n", err)
		_ = testDB.Cleanup()
		os.Exit(1)
	}

	code := m.Run()

	if err := testDB.Cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup clickhouse test db: %v\n", err)
	}
	os.Exit(code)
}
