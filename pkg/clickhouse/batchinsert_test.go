package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

const testTable = "batch_inserter_test"

type row struct {
	ID int64 `ch:"id"`
}

var testConn driver.Conn

func TestMain(m *testing.M) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{"localhost:9009"},
		Auth: ch.Auth{
			Database: "default",
			Username: os.Getenv("GONOTELM_CLICKHOUSE_USERNAME"),
			Password: os.Getenv("GONOTELM_CLICKHOUSE_PASSWORD"),
		},
		DialContext: func(ctx context.Context, addr string) (net.Conn, error) {
			dialer := net.Dialer{Timeout: 10 * time.Second}
			return dialer.DialContext(ctx, "tcp", addr)
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open clickhouse: %v\n", err)
		os.Exit(1)
	}
	testConn = conn

	ctx := context.Background()
	if err := conn.Ping(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ping clickhouse: %v\n", err)
		os.Exit(1)
	}
	if err := conn.Exec(ctx, "CREATE TABLE IF NOT EXISTS "+testTable+" (id Int64) ENGINE=Memory"); err != nil {
		fmt.Fprintf(os.Stderr, "create table: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = conn.Exec(ctx, "DROP TABLE IF EXISTS "+testTable)
	_ = conn.Close()
	os.Exit(code)
}

func truncateTestTable(t *testing.T) {
	t.Helper()
	if err := testConn.Exec(context.Background(), "TRUNCATE TABLE "+testTable); err != nil {
		t.Fatal(err)
	}
}

func countIDs(t *testing.T) []int64 {
	t.Helper()
	rows, err := testConn.Query(context.Background(), "SELECT id FROM "+testTable+" ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestNewBatchInserter_InvalidParams(t *testing.T) {
	ctx := context.Background()
	if _, err := NewBatchInserter(ctx, nil, "INSERT INTO t", -1, time.Millisecond); err == nil {
		t.Fatal("expected chanSize error")
	}
	if _, err := NewBatchInserter(ctx, nil, "INSERT INTO t", 1, 0); err == nil {
		t.Fatal("expected interval error")
	}
}

func TestBatchInserter_CloseDrainsAll(t *testing.T) {
	truncateTestTable(t)
	ctx := context.Background()

	bi, err := NewBatchInserter(ctx, testConn, "INSERT INTO "+testTable+"(id)", 64, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	const n = 200
	for i := int64(0); i < n; i++ {
		if err := bi.Append(t.Context(), &row{ID: i}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if err := bi.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bi.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	got := countIDs(t)
	if len(got) != n {
		t.Fatalf("lost data: got %d rows want %d", len(got), n)
	}
	for i := int64(0); i < n; i++ {
		if got[i] != i {
			t.Fatalf("missing/dup at %d: %v", i, got)
		}
	}
}

func TestBatchInserter_AppendAfterClose(t *testing.T) {
	truncateTestTable(t)
	ctx := context.Background()

	bi, err := NewBatchInserter(ctx, testConn, "INSERT INTO "+testTable+"(id)", 8, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := bi.Close(); err != nil {
		t.Fatal(err)
	}
	if err := bi.Append(t.Context(), &row{ID: 1}); !errors.Is(err, ErrBatcherIsClosed) {
		t.Fatalf("got %v want ErrBatcherIsClosed", err)
	}
}

func TestBatchInserter_ConcurrentAppendNoLoss(t *testing.T) {
	truncateTestTable(t)
	ctx := context.Background()

	bi, err := NewBatchInserter(ctx, testConn, "INSERT INTO "+testTable+"(id)", 64, 20*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	const (
		workers = 16
		per     = 100
		total   = workers * per
	)

	var wg sync.WaitGroup
	errCh := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(base int64) {
			defer wg.Done()
			for i := int64(0); i < per; i++ {
				id := base + i
				if err := bi.Append(t.Context(), &row{ID: id}); err != nil {
					errCh <- err
					return
				}
			}
		}(int64(w * per))
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("append: %v", err)
	}

	if err := bi.Close(); err != nil {
		t.Fatal(err)
	}

	got := countIDs(t)
	if len(got) != total {
		t.Fatalf("data loss: got %d want %d", len(got), total)
	}
	for i := int64(0); i < total; i++ {
		if got[i] != i {
			t.Fatalf("missing/dup around %d: %v", i, got)
		}
	}
}

func TestBatchInserter_ConcurrentCloseAndAppend(t *testing.T) {
	truncateTestTable(t)
	ctx := context.Background()

	bi, err := NewBatchInserter(ctx, testConn, "INSERT INTO "+testTable+"(id)", 32, 10*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 8
	const per = 200

	var (
		wg       sync.WaitGroup
		acceptMu sync.Mutex
		accepted []int64
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(5 * time.Millisecond)
		if err := bi.Close(); err != nil {
			t.Errorf("close: %v", err)
		}
	}()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(base int64) {
			defer wg.Done()
			for i := int64(0); i < per; i++ {
				id := base*per + i
				err := bi.Append(t.Context(), &row{ID: id})
				switch {
				case err == nil:
					acceptMu.Lock()
					accepted = append(accepted, id)
					acceptMu.Unlock()
				case errors.Is(err, ErrBatcherIsClosed):
				default:
					t.Errorf("append %d: %v", id, err)
					return
				}
			}
		}(int64(w))
	}
	wg.Wait()

	got := countIDs(t)
	want := append([]int64(nil), accepted...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })

	if len(got) != len(want) {
		t.Fatalf("persisted=%d accepted=%d (loss or phantom)", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mismatch at %d: persisted=%d accepted=%d", i, got[i], want[i])
		}
	}
}
