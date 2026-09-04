package exchange

import (
	"context"
	"errors"
	"sync"

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
// handler was subscribed to.
type EventHandler[E any] func(topic Topic, event E)

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

// options holds the runtime configuration of an Exchange.
type options struct {
	maxConcurrency int
}

// Option configures an Exchange at creation time.
type Option func(*options)

// WithMaxConcurrency caps the number of event handlers running
// concurrently, enforced by the internal ants goroutine pool that
// executes every handler. While all workers are busy, Publish blocks,
// which provides back pressure. The value is passed straight to
// ants.NewPool: non-positive values mean unlimited, in which case the
// pool spawns a new worker for every event.
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
	pool *ants.Pool    // executes every handler; size -1 means unlimited

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
	e := &Exchange[E]{
		subsByTopic:  make(map[Topic][]*subscription[E]),
		subsByHandle: make(map[Handle]*subscription[E]),
		done:         make(chan struct{}),
	}

	var cfg options
	for _, opt := range opts {
		opt(&cfg)
	}

	// Every handler runs on the pool; maxConcurrency is passed straight
	// through, with ants mapping non-positive sizes to "unlimited".
	pool, err := ants.NewPool(cfg.maxConcurrency)
	if err != nil {
		panic("exchange: failed to create pool: " + err.Error())
	}
	e.pool = pool
	return e
}

// Subscribe registers handler to be invoked on every event published to
// topic.
//
// The returned handle can be passed to Unsubscribe to cancel the
// subscription. Subscribe returns ErrClosed once Terminate has been called.
func (e *Exchange[E]) Subscribe(topic Topic, handler EventHandler[E]) (Handle, error) {
	if handler == nil {
		return 0, errors.New("exchange: nil handler")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.state != stateRunning {
		return 0, ErrClosed
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
// The subscribed handlers are snapshotted before delivery starts: handlers
// subscribed or unsubscribed concurrently may or may not take part in this
// delivery. Handlers are free to re-enter Subscribe, Unsubscribe and
// Publish; doing so cannot deadlock.
//
// If the pool rejects a task (for instance after the pool has been
// released), the affected handler is not invoked and the returned error
// reports the failure; the remaining handlers still run.
//
// Publish returns ErrClosed once Terminate has been called.
func (e *Exchange[E]) Publish(topic Topic, event E) error {
	e.mu.RLock()
	if e.state != stateRunning {
		e.mu.RUnlock()
		return ErrClosed
	}
	// Snapshot the handlers and count them in the WaitGroup while still
	// holding the read lock: Terminate acquires the write lock before it
	// waits on the WaitGroup, so every handler counted here is guaranteed
	// to be waited for.
	subs := e.subsByTopic[topic]
	handlers := make([]EventHandler[E], len(subs))
	for i, sub := range subs {
		handlers[i] = sub.handler
	}
	e.wg.Add(len(handlers))
	e.mu.RUnlock()

	// Submit outside the lock: the back pressure of a saturated pool must
	// not block subscription changes, and re-entrant handler calls must
	// not deadlock against the read lock held here.
	var err error
	for _, handler := range handlers {
		if serr := e.submit(handler, topic, event); serr != nil {
			err = errors.Join(err, serr)
		}
	}
	return err
}

// PublishSync is like Publish, but it invokes the matching handlers one
// after another, synchronously in the calling goroutine, and returns only
// once they have all completed. Handler panics are recovered.
//
// PublishSync holds the exchange's read lock while the handlers run, so a
// concurrent Terminate waits for it to complete.
//
// PublishSync returns ErrClosed once Terminate has been called.
func (e *Exchange[E]) PublishSync(topic Topic, event E) error {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.state != stateRunning {
		return ErrClosed
	}

	for _, sub := range e.subsByTopic[topic] {
		e.runHandler(sub.handler, topic, event)
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
// internal goroutine pool is only released once a Terminate call runs to
// completion, so a timed-out Terminate must be retried (with a fresh
// context) until it returns nil.
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
	e.mu.Unlock()

	// All handlers have finished; stop the pool's background purger.
	e.pool.Release()
	return nil
}

// submit runs handler on the goroutine pool that executes every event
// handler. With MaxConcurrency set, Submit blocks while all workers are
// busy, which is what provides the back pressure.
//
// The caller must already have counted the handler in the WaitGroup, under
// the read lock, so that the count cannot race with the WaitGroup.Wait in
// Terminate. If the pool rejects the task, submit releases the count again
// and returns the error.
func (e *Exchange[E]) submit(handler EventHandler[E], topic Topic, event E) error {
	if err := e.pool.Submit(func() {
		defer e.wg.Done()
		e.runHandler(handler, topic, event)
	}); err != nil {
		// The task never ran; release the counter again.
		e.wg.Done()
		return err
	}
	return nil
}

// runHandler invokes handler, recovering from a panic: a faulty handler
// must not take down the pool worker or the PublishSync caller. The panic
// value is deliberately discarded.
func (e *Exchange[E]) runHandler(handler EventHandler[E], topic Topic, event E) {
	defer func() { _ = recover() }()
	handler(topic, event)
}
