package deptest

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func TestClickHouse(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9009"},
		Auth: clickhouse.Auth{
			Database: "default",
			Username: os.Getenv("GONOTELM_CLICKHOUSE_USERNAME"),
			Password: os.Getenv("GONOTELM_CLICKHOUSE_PASSWORD"),
		},
		DialContext: func(ctx context.Context, addr string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: time.Second * 10}
			return dialer.DialContext(ctx, "tcp", addr)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	serverVersion, err := conn.ServerVersion()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(serverVersion)
}

func TestChanFull(t *testing.T) {
	ch := make(chan int, 10)
	for i := range 11 {
		select {
		case ch <- i:
		default:
			t.Logf("full, len = %d\n", len(ch))
		}
	}
}

func TestBatch(t *testing.T) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"localhost:9009"},
		Auth: clickhouse.Auth{
			Database: "gonotelm",
			Username: os.Getenv("GONOTELM_CLICKHOUSE_USERNAME"),
			Password: os.Getenv("GONOTELM_CLICKHOUSE_PASSWORD"),
		},
		DialContext: func(ctx context.Context, addr string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: time.Second * 10}
			return dialer.DialContext(ctx, "tcp", addr)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = conn.Exec(t.Context(), "CREATE TABLE IF NOT EXISTS temp_test (id Int64, name String) ENGINE=Memory")
	if err != nil {
		t.Fatal(err)
	}

	type user struct {
		Id   int64  `ch:"id"`
		Name string `ch:"name"`
	}

	batch, err := conn.PrepareBatch(t.Context(), "INSERT INTO temp_test(id, name)")
	if err != nil {
		t.Fatal(err)
	}

	for id := range int64(20) {
		err = batch.AppendStruct(&user{Id: id, Name: uuid.NewV4().String()})
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Log(batch.Rows())
	err = batch.Send()
	t.Log(err)
	err = batch.Close()
	t.Log(err)

	err = conn.Exec(t.Context(), "DROP TABLE temp_test")
	t.Log(err)
}
