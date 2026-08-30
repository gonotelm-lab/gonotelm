package clickhouse

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/olap/schema"

	ch "github.com/ClickHouse/clickhouse-go/v2"
)

var testDriver ch.Conn

var testTables = []struct {
	name   string
	schema string
}{
	{schema.LLMLogTableName, schema.LLMLogSchema},
	{schema.EmbeddingLogTableName, schema.EmbeddingLogSchema},
	{schema.Text2ImageLogTableName, schema.Text2ImageLogSchema},
	{schema.Text2AudioLogTableName, schema.Text2AudioLogSchema},
}

func openTestConn(database string) (ch.Conn, error) {
	return ch.Open(&ch.Options{
		Addr: []string{os.Getenv("GONOTELM_CLICKHOUSE_ADDR")},
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
	for _, t := range testTables {
		if err := admin.Exec(ctx, "DROP TABLE IF EXISTS "+t.name); err != nil {
			fmt.Fprintf(os.Stderr, "drop table %s: %v\n", t.name, err)
			os.Exit(1)
		}
		if err := admin.Exec(ctx, t.schema); err != nil {
			fmt.Fprintf(os.Stderr, "create table %s: %v\n", t.name, err)
			os.Exit(1)
		}
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

	for _, t := range testTables {
		_ = testDriver.Exec(ctx, "DROP TABLE IF EXISTS "+t.name)
	}
	_ = testDriver.Close()
	os.Exit(code)
}
