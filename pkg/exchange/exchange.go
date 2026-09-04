package exchange

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	pkglog "github.com/gonotelm-lab/gonotelm/pkg/log"
	"github.com/panjf2000/ants/v2"
)

// Topic identifies a kind of event. Handlers are registered against exact
// topic names: Publish delivers an event only to the handlers subscribed to
// the very same topic.
type Topic = string

// Handle identifies a single subscription. It is returned by Subscribe and
// can be passed to Unsubscribe to cancel the subscription.
type Handle uint64

// EventHandler is invoked for every event published to the topic the
// handler was subscribed to. The ctx comes from the publishing call:
// Publish detaches it via context.WithoutCancel (handlers outlive the
// publisher, so they must not be aborted when it is cancelled, while
// values such as trace metadata still propagate), PublishSync passes it
// through as-is.
type EventHandler[E any] func(ctx context.Context, topic Topic, event E)

// Exchange lifecycle states.
const (
	stateRunning = iota
	stateTerminating
	stateTerminated
)

// Errors returned by the exchange methods.
var (
	// ErrClosed is returned by Subscribe and Publish once Terminate has
	// been called.
	ErrClosed = errors.New("exchange is closed")

	// ErrInvalidHandle is returned by Unsubscribe when the handle does not
	// refer to an active subscription — for instance because it has
	// already been cancelled.
	ErrInvalidHandle = errors.New("invalid handle")
)

// newPool is an indirection over ants.NewPool so tests can inject failures.
var newPool = ants.NewPool

// options holds the runtime configuration of an Exchange.
type options struct {
	maxConcurrency int
}

// Option configures an Exchange at creation time.
type Option func(*options)

// WithMaxConcurrency caps the number of event handlers running
// concurrently per topic, enforced by the ants goroutine pool that each
// topic gets on its first subscription. While a topic's workers are all
// busy, Publish blocks for that topic only — other topics are unaffected.
// The value is passed straight to ants.NewPool: non-positive values mean
// unlimited, in which case the pool spawns a new worker for every event.
func WithMaxConcurrency(n int) Option {
	return func(o *options) { o.maxConcurrency = n }
}

// subscription is a single registered handler.
type subscription[E any] struct {
	handle  Handle
	topic   Topic
	handler EventHandler[E]
}

// Exchange is an in-process message exchange implementing the
// publish-subscribe messaging pattern.
//
// Handlers run asynchronously in their own goroutines, except when invoked
// through PublishSync. The zero value is not usable; create an Exchange
// with New.
//
// All methods are safe for concurrent use.
type Exchange[E any] struct {
	mu sync.RWMutex

	// Forward index: topic → subscriptions under that topic. Publish
	// walks it to deliver events.
	subsByTopic map[Topic][]*subscription[E]
	// Reverse index: handle → subscription. Lets Unsubscribe locate a
	// subscription in constant time.
	subsByHandle map[Handle]*subscription[E]

	nextHandle Handle // handle to assign to the next subscription

	state int // lifecycle state: running / terminating / terminated

	done chan struct{} // closed once no handler is running after Terminate

	// One goroutine pool per topic, created lazily on the topic's first
	// subscription. Entries live until Terminate even if the topic's last
	// subscription is cancelled, so that queued tasks are never rejected
	// mid-flight.
	pools map[Topic]*ants.Pool
	// Cap passed to every topic pool; ants maps non-positive sizes to
	// "unlimited".
	maxConcurrency int

	wg sync.WaitGroup // tracks the number of running handlers
}

// New creates a new Exchange. The type parameter is the event type, e.g.
//
//	ex := exchange.New[string]()
//
// or, to keep using interface{}-style events,
//
//	ex := exchange.New[any]()
func New[E any](opts ...Option) *Exchange[E] {
	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}

	e := &Exchange[E]{
		subsByTopic:    make(map[Topic][]*subscription[E]),
		subsByHandle:   make(map[Handle]*subscription[E]),
		done:           make(chan struct{}),
		pools:          make(map[Topic]*ants.Pool),
		maxConcurrency: cfg.maxConcurrency,
	}
	return e
}

// Subscribe registers handler to be invoked on every event published to
// topic.
//
// The returned handle can be passed to Unsubscribe to cancel the
// subscription. Subscribe returns ErrClosed once Terminate has been
// called, and an error if the topic's goroutine pool cannot be created.
// It never blocks.
func (e *Exchange[E]) Subscribe(topic Topic, handler EventHandler[E]) (Handle, error) {
	if handler == nil {
		return 0, errors.New("exchange: nil handler")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != stateRunning {
		return 0, ErrClosed
	}

	// Give the topic its own pool on first subscription so its back
	// pressure can never block other topics. Create it before recording
	// the subscription so a failure leaves no state behind.
	if _, ok := e.pools[topic]; !ok {
		pool, err := newPool(e.maxConcurrency, ants.WithLogger(pkglog.NewAntsLogger(nil)))
		if err != nil {
			return 0, fmt.Errorf("exchange: create pool for topic %q: %w", topic, err)
		}
		e.pools[topic] = pool
	}

	sub := &subscription[E]{
		handle:  e.nextHandle,
		topic:   topic,
		handler: handler,
	}
	e.nextHandle++

	// Record the subscription in both indexes: the forward one for
	// delivery, the reverse one for cancellation.
	e.subsByTopic[topic] = append(e.subsByTopic[topic], sub)
	e.subsByHandle[sub.handle] = sub
	return sub.handle, nil
}

// Unsubscribe cancels the subscription identified by handle.
//
// It returns ErrInvalidHandle when the handle is unknown or has already
// been cancelled.
func (e *Exchange[E]) Unsubscribe(handle Handle) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Look the subscription up through the reverse index, then use the
	// topic recorded on it to find the forward list it lives in.
	sub, ok := e.subsByHandle[handle]
	if !ok {
		return ErrInvalidHandle
	}
	delete(e.subsByHandle, handle)

	// Remove the subscription from its topic list.
	subs := e.subsByTopic[sub.topic]
	for i, s := range subs {
		if s == sub {
			subs = append(subs[:i], subs[i+1:]...)
			break
		}
	}
	// Drop the topic key once its last subscription is gone, so that the
	// map does not grow without bound over time.
	if len(subs) == 0 {
		delete(e.subsByTopic, sub.topic)
	} else {
		e.subsByTopic[sub.topic] = subs
	}
	return nil
}

// Publish delivers event to every handler subscribed to the given topic.
//
// Each handler runs in its own goroutine, so Publish does not wait for
// handlers to complete — unless WithMaxConcurrency is set and the cap is
// reached, in which case Publish blocks until a worker frees up.
//
// Handlers receive ctx detached via context.WithoutCancel: values such as
// trace metadata propagate, but cancellation does not — delivery is
// asynchronous and outlives the publishing call, so a cancelled publisher
// must not abort handlers that are already running or queued.
//
// The subscribed handlers are snapshotted before delivery starts: handlers
// subscribed or unsubscribed concurrently may or may not take part in this
// delivery. Handlers are free to re-enter Subscribe, Unsubscribe and
// Publish; doing so cannot deadlock.
//
// Delivery runs on the topic's own goroutine pool, so back pressure on one
// topic never blocks publishes to another topic. If the pool rejects a
// task (for instance after the pool has been released), the affected
// handler is not invoked and the returned error reports the failure; the
// remaining handlers still run.
//
// Publish returns ErrClosed once Terminate has been called.
func (e *Exchange[E]) Publish(ctx context.Context, topic Topic, event E) error {
	e.mu.RLock()
	if e.state != stateRunning {
		e.mu.RUnlock()
		return ErrClosed
	}
	// Snapshot the handlers — and the topic pool they run on — and count
	// them in the WaitGroup while still holding the read lock: Terminate
	// acquires the write lock before it waits on the WaitGroup, so every
	// handler counted here is guaranteed to be waited for.
	subs := e.subsByTopic[topic]
	handlers := make([]EventHandler[E], len(subs))
	for i, sub := range subs {
		handlers[i] = sub.handler
	}
	pool := e.pools[topic] // non-nil whenever the topic has subscriptions
	e.wg.Add(len(handlers))
	e.mu.RUnlock()

	// Handlers outlive the publishing call: strip the cancellation but
	// keep the values.
	hctx := context.WithoutCancel(ctx)

	// Submit outside the lock: the back pressure of a saturated pool must
	// not block subscription changes, and re-entrant handler calls must
	// not deadlock against the read lock held here.
	var err error
	for _, handler := range handlers {
		if serr := e.submit(hctx, pool, handler, topic, event); serr != nil {
			err = errors.Join(err, serr)
		}
	}
	return err
}

// PublishSync is like Publish, but it invokes the matching handlers one
// after another, synchronously in the calling goroutine, and returns only
// once they have all completed. Handler panics are recovered.
//
// Unlike Publish, the ctx is passed through to the handlers as-is: a
// synchronous handler is part of the calling call chain and honours its
// cancellation.
//
// PublishSync holds the exchange's read lock while the handlers run, so a
// concurrent Terminate waits for it to complete.
//
// PublishSync returns ErrClosed once Terminate has been called.
func (e *Exchange[E]) PublishSync(ctx context.Context, topic Topic, event E) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.state != stateRunning {
		return ErrClosed
	}

	for _, sub := range e.subsByTopic[topic] {
		e.runHandler(ctx, sub.handler, topic, event)
	}
	return nil
}

// Wait blocks until every handler that is running at the moment of the call
// has completed. It does not wait for handlers started by subsequent
// Publish calls, nor does it prevent them. For a graceful shutdown use
// Terminate instead.
func (e *Exchange[E]) Wait() {
	e.wg.Wait()
}

// WaitContext is like Wait, but it returns ctx.Err() if the running
// handlers do not finish before ctx is done.
func (e *Exchange[E]) WaitContext(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Terminate makes it impossible to publish any further events and waits
// for all running handlers to return.
//
// It is safe to call Terminate multiple times; once termination has been
// initiated, subsequent calls wait for the ongoing termination to complete
// (bounded by their own ctx) and then return nil.
//
// If ctx expires before all handlers have finished, Terminate returns
// ctx.Err(). The exchange is closed to new events either way, but the
// topic pools are only released once a Terminate call runs to completion,
// so a timed-out Terminate must be retried (with a fresh context) until it
// returns nil.
//
// When ctx carries a deadline, the pools are torn down with
// ReleaseTimeout within the remaining budget, so a nil return additionally
// guarantees that every worker goroutine has exited. If that wait times
// out, Terminate reports it as the error result — the exchange is closed
// and all handlers have run to completion either way.
func (e *Exchange[E]) Terminate(ctx context.Context) error {
	e.mu.Lock()
	switch e.state {
	case stateRunning:
		e.state = stateTerminating
		// No new handlers can be dispatched from now on, so the
		// WaitGroup counter can only decrease; waiting for it in the
		// background is therefore safe.
		go func() {
			e.wg.Wait()
			close(e.done)
		}()
	case stateTerminated:
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	select {
	case <-e.done:
	case <-ctx.Done():
		return ctx.Err()
	}

	e.mu.Lock()
	if e.state == stateTerminating {
		e.state = stateTerminated
	}
	// All handlers have finished; snapshot the topic pools and stop their
	// background purgers outside the lock. No pool can be created from
	// now on (state is terminated), so the map is frozen and the teardown
	// cannot race with Subscribe.
	pools := make([]*ants.Pool, 0, len(e.pools))
	for _, p := range e.pools {
		pools = append(pools, p)
	}
	e.mu.Unlock()

	return e.releasePools(ctx, pools)
}

// releasePools tears the topic pools down after every handler has
// finished. With a deadline on ctx it uses ReleaseTimeout, so that a nil
// return guarantees the worker goroutines exited within the remaining
// budget; otherwise plain Release closes the pools and lets the workers
// exit on their own. ErrPoolClosed — a pool already closed by a concurrent
// or repeated teardown — is treated as success.
func (e *Exchange[E]) releasePools(ctx context.Context, pools []*ants.Pool) error {
	var timeout time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		if timeout = time.Until(deadline); timeout < 0 {
			timeout = 0
		}
	}

	var err error
	for _, p := range pools {
		if timeout == 0 {
			p.Release()
			continue
		}
		if rerr := p.ReleaseTimeout(timeout); rerr != nil && !errors.Is(rerr, ants.ErrPoolClosed) {
			err = errors.Join(err, rerr)
		}
	}
	return err
}

// submit runs handler on the topic's goroutine pool. While all of the
// topic's workers are busy, Submit blocks, which is what provides the back
// pressure — for this topic only.
//
// The caller must already have counted the handler in the WaitGroup, under
// the read lock, so that the count cannot race with the WaitGroup.Wait in
// Terminate. If the pool rejects the task, submit releases the count again
// and returns the error.
func (e *Exchange[E]) submit(ctx context.Context, pool *ants.Pool, handler EventHandler[E], topic Topic, event E) error {
	if err := pool.Submit(func() {
		defer e.wg.Done()
		e.runHandler(ctx, handler, topic, event)
	}); err != nil {
		// The task never ran; release the counter again.
		e.wg.Done()
		return err
	}
	return nil
}

// runHandler invokes handler, recovering from a panic: a faulty handler
// must not take down the pool worker or the PublishSync caller.
func (e *Exchange[E]) runHandler(ctx context.Context, handler EventHandler[E], topic Topic, event E) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "exchange: event handler panic",
				"topic", topic,
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()
	handler(ctx, topic, event)
}
