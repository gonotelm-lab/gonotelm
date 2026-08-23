package clickhouse

import (
	"context"
	"fmt"
	"os"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

const testTable = "llm_logs"

const testTableDDL = `
CREATE TABLE IF NOT EXISTS llm_logs (
	id String,
	group_id String,
	trace_id String,
	user_id String,
	scene LowCardinality(String),
	model LowCardinality(String),
	model_provider LowCardinality(String),
	model_parameters Nullable(String),
	call_start_time DateTime64(3),
	call_finish_time DateTime64(3),
	input Nullable(String),
	output Nullable(String),
	tool_definitions Map(LowCardinality(String), String),
	tool_calls Array(Tuple(name LowCardinality(String), arguments String)),
	usage_details Map(LowCardinality(String), UInt64),
	cost_details Map(LowCardinality(String), Decimal64(12)),
	total_cost Nullable(Decimal64(12)),
	create_time DateTime64(3),
	update_time DateTime64(3),
	metadata Map(LowCardinality(String), String),
	error Nullable(String),
	is_deleted UInt8
) ENGINE = Memory`

var testDriver ch.Conn

func openTestConn(database string) (ch.Conn, error) {
	return ch.Open(&ch.Options{
		Addr: []string{"localhost:9009"},
		Auth: ch.Auth{
			Database: database,
			Username: os.Getenv("GONOTELM_CLICKHOUSE_USERNAME"),
			Password: os.Getenv("GONOTELM_CLICKHOUSE_PASSWORD"),
		},
	})
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	admin, err := openTestConn("default")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open clickhouse: %v\n", err)
		os.Exit(1)
	}
	if err := admin.Exec(ctx, "DROP TABLE IF EXISTS "+testTable); err != nil {
		fmt.Fprintf(os.Stderr, "drop table: %v\n", err)
		os.Exit(1)
	}
	if err := admin.Exec(ctx, testTableDDL); err != nil {
		fmt.Fprintf(os.Stderr, "create table: %v\n", err)
		os.Exit(1)
	}
	_ = admin.Close()

	testDriver, err = openTestConn("default")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open clickhouse: %v\n", err)
		os.Exit(1)
	}
	if err := testDriver.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping clickhouse: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = testDriver.Exec(ctx, "DROP TABLE IF EXISTS "+testTable)
	_ = testDriver.Close()
	os.Exit(code)
}
