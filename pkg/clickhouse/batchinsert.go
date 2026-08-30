package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gonotelm-lab/gonotelm/pkg/trace/instrumentation/clickhouseconv"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

var ErrBatcherIsClosed = errors.New("clickhouse batcher is closed")

type BatchInserter struct {
	conn     ch.Conn
	query    string
	batcher  driver.Batch // only touched by worker; not thread-safe
	dataCh   chan any
	sendCh   chan struct{} // backpressure: request worker to commit current batch
	interval time.Duration
	rootCtx  context.Context

	mu        sync.Mutex
	closed    bool
	closeCh   chan struct{}
	closeOnce sync.Once
	appendWg  sync.WaitGroup // in-flight Append past the closed check
	wg        sync.WaitGroup // worker
	err       error
}

func NewBatchInserter(ctx context.Context, conn ch.Conn, query string, chanSize int, interval time.Duration) (*BatchInserter, error) {
	if chanSize <= 0 {
		return nil, fmt.Errorf("clickhouse batcher: chanSize must be > 0")
	}
	if interval <= 0 {
		return nil, fmt.Errorf("clickhouse batcher: interval must be > 0")
	}

	b := &BatchInserter{
		conn:     conn,
		query:    query,
		dataCh:   make(chan any, chanSize),
		sendCh:   make(chan struct{}, 1),
		closeCh:  make(chan struct{}),
		interval: interval,
		rootCtx:  ctx,
	}

	b.wg.Add(1)
	go b.worker(b.rootCtx)

	return b, nil
}

// Close stops accepting new rows, waits for in-flight Append and the worker to
// drain + Send, then returns the first worker-side error if any.
// Safe to call multiple times.
func (p *BatchInserter) Close() error {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()

		close(p.closeCh)
		p.appendWg.Wait() // every Append that passed the gate has finished send or aborted
		close(p.dataCh)   // seal queue so worker can range to completion
	})
	p.wg.Wait()
	return p.err
}

// Append queues a struct value for insert. val must be compatible with
// AppendStruct (typically a pointer to struct). Blocks when the queue is full
// until ctx is done, the batcher is closed, or space is available.
func (p *BatchInserter) Append(ctx context.Context, val any) (err error) {
	tracer := otel.Tracer("gonotelm/pkg/clickhouse")
	ctx, span := tracer.Start(ctx, clickhouseconv.AppendSpanName(p.query),
		clickhouseconv.AppendSpanStartOpts(p.query)...,
	)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return ErrBatcherIsClosed
	}
	p.appendWg.Add(1)
	p.mu.Unlock()
	defer p.appendWg.Done()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.rootCtx.Done():
		return p.rootCtx.Err()
	case <-p.closeCh:
		return ErrBatcherIsClosed
	case p.dataCh <- val:
		return nil
	default:
		// queue is full: ask worker to commit buffered rows before blocking
		p.requestSend()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.rootCtx.Done():
		return p.rootCtx.Err()
	case <-p.closeCh:
		return ErrBatcherIsClosed
	case p.dataCh <- val:
		return nil
	}
}

func (p *BatchInserter) requestSend() {
	select {
	case p.sendCh <- struct{}{}:
	default:
	}
}

func (p *BatchInserter) worker(ctx context.Context) {
	defer p.wg.Done()

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Parent cancel: drain whatever is already queued, then finalize.
			// Rows still being Append-ed may be rejected via closeCh only after Close();
			// on bare cancel we best-effort drain the channel without sealing it.
			p.drainAvailable()
			p.send()
			return
		case <-p.closeCh:
			// dataCh will be closed by Close after in-flight Appends finish.
			p.consumeRemaining()
			p.send()
			return
		case data, ok := <-p.dataCh:
			if !ok {
				p.send()
				return
			}
			p.appendStruct(data)
		case <-ticker.C:
			p.flush()
		case <-p.sendCh:
			p.flush()
		}
	}
}

// consumeRemaining reads until dataCh is closed and empty.
func (p *BatchInserter) consumeRemaining() {
	for data := range p.dataCh {
		p.appendStruct(data)
	}
}

// drainAvailable non-blocking drain for ctx cancel (channel not sealed).
func (p *BatchInserter) drainAvailable() {
	for {
		select {
		case data, ok := <-p.dataCh:
			if !ok {
				return
			}
			p.appendStruct(data)
		default:
			return
		}
	}
}

func (p *BatchInserter) ensureBatch() error {
	if p.batcher != nil && !p.batcher.IsSent() {
		return nil
	}
	batch, err := p.conn.PrepareBatch(p.rootCtx, p.query)
	if err != nil {
		return fmt.Errorf("clickhouse prepare batch err: %w", err)
	}
	p.batcher = batch
	return nil
}

func (p *BatchInserter) appendStruct(data any) {
	if err := p.ensureBatch(); err != nil {
		p.err = err
		slog.Error("batch inserter prepare batch failed", slog.Any("err", err))
		return
	}
	if err := p.batcher.AppendStruct(data); err != nil {
		p.err = fmt.Errorf("clickhouse append struct: %w", err)
		slog.Error("batch inserter append struct failed", slog.Any("err", err))
	}
}

// flush commits buffered rows via Send(). Flush() only ships blocks without
// closing the INSERT query, so rows are not visible until Send() completes.
func (p *BatchInserter) flush() {
	if p.batcher == nil || p.batcher.IsSent() || p.batcher.Rows() == 0 {
		return
	}
	if err := p.batcher.Send(); err != nil {
		p.err = fmt.Errorf("clickhouse send: %w", err)
		slog.Error("batch inserter send failed", slog.Any("err", err))
		return
	}
	p.batcher = nil
}

func (p *BatchInserter) send() {
	if p.batcher == nil || p.batcher.IsSent() || p.batcher.Rows() == 0 {
		p.batcher = nil
		return
	}
	if err := p.batcher.Send(); err != nil {
		p.err = fmt.Errorf("clickhouse send: %w", err)
		slog.Error("batch inserter send failed", slog.Any("err", err))
		return
	}
	p.batcher = nil
}
