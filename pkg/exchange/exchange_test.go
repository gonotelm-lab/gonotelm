package exchange

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
)

// newTestExchange creates a plain exchange for tests that do not care
// about options.
func newTestExchange() *Exchange[any] {
	return New[any]()
}

func TestExchange_Subscribe(t *testing.T) {
	t.Run("nil handler is rejected", func(t *testing.T) {
		ex := newTestExchange()
		if _, err := ex.Subscribe("topic", nil); err == nil {
			t.Fatal("Subscribe with a nil handler should fail")
		}
	})

	t.Run("handles are unique and increasing", func(t *testing.T) {
		ex := newTestExchange()

		seen := make(map[Handle]bool)
		for range 3 {
			h, err := ex.Subscribe("topic", func(Topic, any) {})
			if err != nil {
				t.Fatal(err)
			}
			if seen[h] {
				t.Fatalf("duplicate handle %d", h)
			}
			seen[h] = true
		}
	})
}

func TestExchange_Publish(t *testing.T) {
	t.Run("handler receives topic and event", func(t *testing.T) {
		ex := newTestExchange()

		const (
			topic   = "chatroom.golang"
			message = "hello"
		)

		got := make(chan string, 1)
		if _, err := ex.Subscribe(topic, func(tp Topic, e any) {
			if tp != topic {
				t.Errorf("unexpected topic %q, want %q", tp, topic)
			}
			got <- e.(string)
		}); err != nil {
			t.Fatal(err)
		}

		if err := ex.Publish(topic, message); err != nil {
			t.Fatal(err)
		}

		select {
		case msg := <-got:
			if msg != message {
				t.Fatalf("event = %q, want %q", msg, message)
			}
		case <-time.After(time.Second):
			t.Fatal("handler never invoked")
		}
	})

	t.Run("topics match exactly", func(t *testing.T) {
		ex := newTestExchange()

		var pushes, pulls atomic.Int32
		ex.Subscribe("git.push", func(Topic, any) { pushes.Add(1) })
		ex.Subscribe("git.pull", func(Topic, any) { pulls.Add(1) })

		// A longer topic must not trigger the "git.push" handler.
		ex.Publish("git.push.force", nil)
		ex.Publish("git.push", nil)
		ex.Publish("git.pull", nil)
		ex.Terminate(context.Background())

		if pushes.Load() != 1 {
			t.Errorf(`"git.push" handler invoked %d times, want 1`, pushes.Load())
		}
		if pulls.Load() != 1 {
			t.Errorf(`"git.pull" handler invoked %d times, want 1`, pulls.Load())
		}
	})

	t.Run("all handlers on a topic are invoked", func(t *testing.T) {
		ex := newTestExchange()

		var count atomic.Int32
		for range 5 {
			if _, err := ex.Subscribe("topic", func(Topic, any) { count.Add(1) }); err != nil {
				t.Fatal(err)
			}
		}

		for range 3 {
			if err := ex.Publish("topic", nil); err != nil {
				t.Fatal(err)
			}
		}
		ex.Terminate(context.Background())

		if got := count.Load(); got != 15 {
			t.Errorf("handlers invoked %d times, want 15", got)
		}
	})

	t.Run("typed events keep their type", func(t *testing.T) {
		type event struct{ ID int }

		ex := New[event]()
		got := make(chan event, 1)
		ex.Subscribe("users.created", func(topic Topic, e event) {
			if topic != "users.created" {
				t.Errorf("unexpected topic %q", topic)
			}
			got <- e
		})

		if err := ex.Publish("users.created", event{ID: 7}); err != nil {
			t.Fatal(err)
		}
		if err := ex.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}

		select {
		case e := <-got:
			if e.ID != 7 {
				t.Errorf("event %+v, want ID 7", e)
			}
		default:
			t.Fatal("handler never invoked")
		}
	})
}

func TestExchange_PublishSync(t *testing.T) {
	// The intentional panic below must be swallowed silently by runHandler.
	ex := New[any]()

	// Handlers run in order, in the calling goroutine, even when one panics.
	var order []string
	ex.Subscribe("topic", func(Topic, any) { order = append(order, "a") })
	ex.Subscribe("topic", func(Topic, any) { order = append(order, "b"); panic("ignored") })
	ex.Subscribe("topic", func(Topic, any) { order = append(order, "c") })

	if err := ex.PublishSync("topic", nil); err != nil {
		t.Fatal(err)
	}

	want := []string{"a", "b", "c"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %v, want %v", order, want)
		}
	}
}

func TestExchange_Unsubscribe(t *testing.T) {
	ex := newTestExchange()

	var count atomic.Int32
	h1, err := ex.Subscribe("topic", func(Topic, any) { count.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	h2, err := ex.Subscribe("topic", func(Topic, any) { count.Add(1) })
	if err != nil {
		t.Fatal(err)
	}
	_ = h2

	if err := ex.Unsubscribe(h1); err != nil {
		t.Fatalf("Unsubscribe(%d) failed: %v", h1, err)
	}
	if err := ex.Unsubscribe(h1); !errors.Is(err, ErrInvalidHandle) {
		t.Errorf("second Unsubscribe = %v, want ErrInvalidHandle", err)
	}
	if err := ex.Unsubscribe(12345); !errors.Is(err, ErrInvalidHandle) {
		t.Errorf("unknown-handle Unsubscribe = %v, want ErrInvalidHandle", err)
	}

	if err := ex.Publish("topic", nil); err != nil {
		t.Fatal(err)
	}
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := count.Load(); got != 1 {
		t.Errorf("handlers invoked %d times, want 1", got)
	}

	t.Run("last unsubscribe removes the topic entry", func(t *testing.T) {
		ex2 := newTestExchange()
		h, _ := ex2.Subscribe("topic", func(Topic, any) {})
		ex2.Unsubscribe(h)

		ex2.mu.Lock()
		_, ok := ex2.subsByTopic["topic"]
		ex2.mu.Unlock()
		if ok {
			t.Error("byTopic entry not removed after the last subscription was cancelled")
		}
	})
}

func TestExchange_Terminate(t *testing.T) {
	t.Run("publish and subscribe fail after terminate", func(t *testing.T) {
		ex := newTestExchange()
		ex.Subscribe("topic", func(Topic, any) {})

		if err := ex.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := ex.Publish("topic", nil); !errors.Is(err, ErrClosed) {
			t.Errorf("Publish after Terminate = %v, want ErrClosed", err)
		}
		if _, err := ex.Subscribe("topic", func(Topic, any) {}); !errors.Is(err, ErrClosed) {
			t.Errorf("Subscribe after Terminate = %v, want ErrClosed", err)
		}
		// Terminate must be idempotent.
		if err := ex.Terminate(context.Background()); err != nil {
			t.Errorf("second Terminate = %v, want nil", err)
		}
	})

	t.Run("waits for running handlers", func(t *testing.T) {
		ex := newTestExchange()
		var finished atomic.Bool
		ex.Subscribe("topic", func(Topic, any) {
			time.Sleep(50 * time.Millisecond)
			finished.Store(true)
		})

		ex.Publish("topic", nil)
		if err := ex.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !finished.Load() {
			t.Error("Terminate returned before the handler finished")
		}
	})

	t.Run("respects context deadline", func(t *testing.T) {
		ex := newTestExchange()
		block := make(chan struct{})
		ex.Subscribe("topic", func(Topic, any) { <-block })
		ex.Publish("topic", nil)

		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if err := ex.Terminate(ctx); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("Terminate = %v, want context.DeadlineExceeded", err)
		}

		// Once the handler unblocks, a repeated Terminate completes and the
		// exchange becomes closed.
		close(block)
		if err := ex.Terminate(context.Background()); err != nil {
			t.Fatal(err)
		}
		if err := ex.Publish("topic", nil); !errors.Is(err, ErrClosed) {
			t.Errorf("Publish after Terminate = %v, want ErrClosed", err)
		}
	})
}

func TestExchange_Wait(t *testing.T) {
	ex := newTestExchange()
	var count atomic.Int32
	ex.Subscribe("topic", func(Topic, any) {
		time.Sleep(50 * time.Millisecond)
		count.Add(1)
	})

	ex.Publish("topic", nil)
	ex.Wait()
	if got := count.Load(); got != 1 {
		t.Errorf("handler count = %d, want 1 after Wait", got)
	}

	// Wait must not prevent further publishes.
	ex.Publish("topic", nil)
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := count.Load(); got != 2 {
		t.Errorf("handler count = %d, want 2", got)
	}
}

func TestExchange_WaitContext(t *testing.T) {
	ex := newTestExchange()
	block := make(chan struct{})
	ex.Subscribe("topic", func(Topic, any) { <-block })
	ex.Publish("topic", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := ex.WaitContext(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("WaitContext = %v, want context.DeadlineExceeded", err)
	}

	close(block)
	ex.Wait()
}

func TestExchange_PanicRecovery(t *testing.T) {
	type panicEvt struct{ n int }

	var reported atomic.Int32
	ex := New[panicEvt]()

	ex.Subscribe("boom", func(Topic, panicEvt) { panic("kaboom") })
	ex.Subscribe("boom", func(Topic, panicEvt) { reported.Add(1000) })

	if err := ex.Publish("boom", panicEvt{n: 42}); err != nil {
		t.Fatal(err)
	}
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}

	// The panicking handler must not take down the healthy one.
	if got := reported.Load(); got != 1000 {
		t.Errorf("second handler reports %d, want 1000", got)
	}
}

func TestExchange_MaxConcurrency(t *testing.T) {
	ex := New[any](WithMaxConcurrency(2))

	var running, maxRunning atomic.Int32
	var wg sync.WaitGroup
	ex.Subscribe("topic", func(Topic, any) {
		cur := running.Add(1)
		for {
			old := maxRunning.Load()
			if cur <= old || maxRunning.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		running.Add(-1)
		wg.Done()
	})

	const n = 20
	wg.Add(n)
	for range n {
		if err := ex.Publish("topic", nil); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}

	if m := maxRunning.Load(); m > 2 {
		t.Errorf("max concurrent handlers = %d, want <= 2", m)
	}
}

func TestExchange_ConcurrentStress(t *testing.T) {
	ex := New[int]()
	handles := make([]Handle, 8)
	for i := range handles {
		h, err := ex.Subscribe("topic", func(Topic, int) { time.Sleep(time.Microsecond) })
		if err != nil {
			t.Fatal(err)
		}
		handles[i] = h
	}

	const workers = 8
	const events = 50
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range events {
				ex.Publish("topic", i)
				ex.Publish("other", i)
			}
		})
	}
	wg.Go(func() {
		for _, h := range handles {
			ex.Unsubscribe(h)
		}
	})
	wg.Wait()

	// Subscribing again must still work.
	if _, err := ex.Subscribe("topic", func(Topic, int) {}); err != nil {
		t.Fatal(err)
	}
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// Regression tests ---------------------------------------------------------------

func TestPublish_HandlerReentrancy_DoesNotDeadlock(t *testing.T) {
	// Regression: Publish used to hold the read lock while blocked on the
	// full pool. A handler re-entering Unsubscribe then deadlocked against
	// it: the worker never freed up, so Publish never released the lock.
	ex := New[any](WithMaxConcurrency(1))

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var unsubscribed atomic.Bool
	var h Handle
	h, err := ex.Subscribe("t", func(Topic, any) {
		once.Do(func() { close(started) })
		<-release
		// May run twice after the fix (the snapshot delivers to the
		// handler again before the unsubscribe lands); the second call
		// just fails with ErrInvalidHandle.
		ex.Unsubscribe(h)
		unsubscribed.Store(true)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Occupy the single worker; the handler parks on release.
	if err := ex.Publish("t", nil); err != nil {
		t.Fatal(err)
	}
	<-started

	// This Publish takes the read lock and parks on the full pool.
	blocked := make(chan error, 1)
	go func() { blocked <- ex.Publish("t", nil) }()
	time.Sleep(20 * time.Millisecond) // let it take the lock and park

	// Unblocking the handler must not deadlock: Unsubscribe needs the
	// lock that Publish (before the fix) was still holding.
	close(release)

	select {
	case err := <-blocked:
		if err != nil {
			t.Fatalf("second Publish = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("deadlock: second Publish never returned while the handler was stuck in Unsubscribe")
	}
	if !unsubscribed.Load() {
		t.Error("handler never completed its Unsubscribe call")
	}

	// Everything drained; shut down cleanly. No defer: Terminate would
	// hang on the deadlocked state this test guards against.
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestTerminate_WaitsForConcurrentPublishSync(t *testing.T) {
	// Locks the contract: PublishSync runs its handlers while holding the
	// exchange's read lock, and Terminate must take that lock before
	// closing, so an in-progress PublishSync always finishes first.
	ex := newTestExchange()

	finished := atomic.Bool{}
	ex.Subscribe("t", func(Topic, any) {
		time.Sleep(300 * time.Millisecond)
		finished.Store(true)
	})

	go ex.PublishSync("t", nil)
	time.Sleep(50 * time.Millisecond) // let PublishSync enter the handler

	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !finished.Load() {
		t.Error("Terminate returned before the concurrent PublishSync completed")
	}
}

func TestTerminate_TimeoutThenRetryReleasesPool(t *testing.T) {
	// Locks the contract for recovering from a timed-out Terminate: the
	// pool must stay open while handlers run, and a retried Terminate must
	// eventually release it.
	ex := New[any]()
	block := make(chan struct{})
	ex.Subscribe("t", func(Topic, any) { <-block })
	if err := ex.Publish("t", nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := ex.Terminate(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Terminate = %v, want context.DeadlineExceeded", err)
	}
	if ex.pools["t"].IsClosed() {
		t.Error("pool released while handlers were still running")
	}

	close(block)
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !ex.pools["t"].IsClosed() {
		t.Error("pool not released after the retried Terminate completed")
	}
}

func TestTerminate_SecondCallWaitsForOngoing(t *testing.T) {
	// Locks the documented behavior: once termination is in flight,
	// subsequent Terminate calls wait for it rather than returning
	// immediately.
	ex := newTestExchange()
	block := make(chan struct{})
	ex.Subscribe("t", func(Topic, any) { <-block })
	if err := ex.Publish("t", nil); err != nil {
		t.Fatal(err)
	}

	done1 := make(chan error, 1)
	go func() { done1 <- ex.Terminate(context.Background()) }()
	time.Sleep(20 * time.Millisecond)

	done2 := make(chan error, 1)
	go func() { done2 <- ex.Terminate(context.Background()) }()
	select {
	case <-done2:
		t.Error("second Terminate returned before the first completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(block)
	if err := <-done1; err != nil {
		t.Fatal(err)
	}
	if err := <-done2; err != nil {
		t.Fatal(err)
	}
}

func TestPublish_ReportsSubmitFailure(t *testing.T) {
	// Regression: a rejected pool task silently dropped the event while
	// Publish reported success to the caller.
	ex := New[any]()
	var called atomic.Bool
	if _, err := ex.Subscribe("t", func(Topic, any) { called.Store(true) }); err != nil {
		t.Fatal(err)
	}
	ex.pools["t"].Release() // force every Submit on this topic to fail

	if err := ex.Publish("t", nil); err == nil {
		t.Fatal("Publish = nil, want an error when the pool rejects the task")
	}
	if called.Load() {
		t.Error("handler invoked despite the submit failure")
	}
}

func TestPublish_Backpressure_BlocksWhenPoolFull(t *testing.T) {
	// Locks the WithMaxConcurrency back pressure contract: while all
	// workers are busy, Publish blocks instead of queueing unboundedly,
	// and returns once a worker frees up.
	ex := New[any](WithMaxConcurrency(1))

	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	var ran atomic.Int32
	ex.Subscribe("t", func(Topic, any) {
		once.Do(func() { close(started) })
		<-release
		ran.Add(1)
	})

	// Occupy the only worker; the handler parks on release.
	if err := ex.Publish("t", nil); err != nil {
		t.Fatal(err)
	}
	<-started

	// A second Publish must block until the worker frees up.
	published := make(chan error, 1)
	go func() { published <- ex.Publish("t", nil) }()
	select {
	case err := <-published:
		t.Fatalf("Publish returned %v while the pool was saturated, want it to block", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	select {
	case err := <-published:
		if err != nil {
			t.Fatalf("Publish = %v, want nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Publish never returned after a worker freed up")
	}

	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := ran.Load(); got != 2 {
		t.Errorf("handlers ran %d times, want 2", got)
	}
}

func TestPublish_TopicIsolation(t *testing.T) {
	// Each topic gets its own goroutine pool: a saturated topic must not
	// block publishes to other topics.
	ex := New[any](WithMaxConcurrency(1))

	block := make(chan struct{})
	started := make(chan struct{})
	var once sync.Once
	ex.Subscribe("slow", func(Topic, any) {
		once.Do(func() { close(started) })
		<-block
	})
	fastRan := make(chan struct{}, 1)
	ex.Subscribe("fast", func(Topic, any) { fastRan <- struct{}{} })

	// Saturate the "slow" topic's pool.
	if err := ex.Publish("slow", nil); err != nil {
		t.Fatal(err)
	}
	<-started

	// "fast" must still deliver immediately.
	done := make(chan error, 1)
	go func() { done <- ex.Publish("fast", nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal(`topic isolation broken: Publish to "fast" blocked behind the saturated "slow" topic`)
	}
	select {
	case <-fastRan:
	case <-time.After(time.Second):
		t.Fatal(`"fast" handler never invoked`)
	}

	close(block)
	if err := ex.Terminate(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestSubscribe_PoolCreationFailure(t *testing.T) {
	// Subscribe must report a topic pool creation failure instead of
	// panicking, and leave no state behind.
	orig := newPool
	t.Cleanup(func() { newPool = orig })
	newPool = func(size int, opts ...ants.Option) (*ants.Pool, error) {
		return nil, errors.New("boom")
	}

	ex := New[any]()
	h, err := ex.Subscribe("t", func(Topic, any) {})
	if err == nil {
		t.Fatal("Subscribe = nil error, want the pool creation failure")
	}
	if h != 0 {
		t.Errorf("handle = %d, want 0 on failure", h)
	}

	ex.mu.Lock()
	_, hasSubs := ex.subsByTopic["t"]
	_, hasPool := ex.pools["t"]
	ex.mu.Unlock()
	if hasSubs || hasPool {
		t.Error("failed Subscribe left subscription or pool state behind")
	}

	// With the failure removed, retrying on the same topic works.
	newPool = orig
	if _, err := ex.Subscribe("t", func(Topic, any) {}); err != nil {
		t.Fatalf("retry Subscribe = %v, want nil", err)
	}
}

// Benchmarks ------------------------------------------------------------------

func BenchmarkExchange_Publish(b *testing.B) {
	ex := New[any]()
	ex.Subscribe("git.push", func(Topic, any) {})

	for b.Loop() {
		ex.Publish("git.push", "b839dc65")
	}

	ex.Wait()
}

func BenchmarkExchange_PublishSync(b *testing.B) {
	ex := New[any]()
	ex.Subscribe("git.push", func(Topic, any) {})

	for b.Loop() {
		ex.PublishSync("git.push", "b839dc65")
	}
}

func BenchmarkExchange_SubscribeUnsubscribe(b *testing.B) {
	ex := New[any]()
	handler := func(Topic, any) {}

	for b.Loop() {
		h, err := ex.Subscribe("git.push", handler)
		if err != nil {
			b.Fatal(err)
		}
		if err := ex.Unsubscribe(h); err != nil {
			b.Fatal(err)
		}
	}
}
