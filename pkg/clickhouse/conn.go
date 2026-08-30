package clickhouse

import (
	"context"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"go.opentelemetry.io/otel/trace"
)

var ErrConnectionClosed = errors.New("clickhouse connection is closed")

type Conn struct {
	conn   ch.Conn
	closed atomic.Bool

	mu       sync.Mutex
	batchers []*BatchInserter
}

func NewConn(conn ch.Conn) *Conn {
	return &Conn{conn: conn}
}

func wrapSpanContext(ctx context.Context) context.Context {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		return ch.Context(ctx, ch.WithSpan(spanContext))
	}

	return ctx
}

func (c *Conn) Ping(ctx context.Context) error {
	if c.closed.Load() {
		return ErrConnectionClosed
	}

	return c.conn.Ping(wrapSpanContext(ctx))
}

func (c *Conn) Select(ctx context.Context, dest any, query string, args ...any) error {
	if c.closed.Load() {
		return ErrConnectionClosed
	}
	return c.conn.Select(wrapSpanContext(ctx), dest, query, args...)
}

func (c *Conn) Query(ctx context.Context, query string, args ...any) (chdriver.Rows, error) {
	if c.closed.Load() {
		return nil, ErrConnectionClosed
	}
	return c.conn.Query(wrapSpanContext(ctx), query, args...)
}

func (c *Conn) QueryRow(ctx context.Context, query string, args ...any) chdriver.Row {
	if c.closed.Load() {
		return pesudoErrRow{err: ErrConnectionClosed}
	}
	return c.conn.QueryRow(wrapSpanContext(ctx), query, args...)
}

func (c *Conn) Exec(ctx context.Context, query string, args ...any) error {
	if c.closed.Load() {
		return ErrConnectionClosed
	}
	return c.conn.Exec(wrapSpanContext(ctx), query, args...)
}

func (c *Conn) QueryFormat(ctx context.Context, format string, query string, args ...any) (io.ReadCloser, error) {
	if c.closed.Load() {
		return nil, ErrConnectionClosed
	}
	return c.conn.QueryFormat(wrapSpanContext(ctx), format, query, args...)
}

func (c *Conn) InsertFormat(ctx context.Context, format string, query string, data io.Reader) error {
	if c.closed.Load() {
		return ErrConnectionClosed
	}
	return c.conn.InsertFormat(wrapSpanContext(ctx), format, query, data)
}

func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return ErrConnectionClosed
	}
	c.closed.Store(true)

	var err error
	for _, b := range c.batchers {
		if closeErr := b.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	if closeErr := c.conn.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	return err
}

func (c *Conn) Stats() chdriver.Stats {
	return c.conn.Stats()
}

func (c *Conn) CreateBatcher(ctx context.Context, query string) (*BatchInserter, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed.Load() {
		return nil, ErrConnectionClosed
	}

	b, err := NewBatchInserter(ctx, c.conn, query, 20000, time.Millisecond*1000)
	if err != nil {
		return nil, err
	}

	c.batchers = append(c.batchers, b)

	return b, nil
}
