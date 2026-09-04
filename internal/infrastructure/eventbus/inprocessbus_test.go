package eventbus

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/pkg/exchange"
)

type testEvent struct {
	*event.BaseInProcessEvent
	category event.Category
	topic    string
	value    any
}

func (e *testEvent) Category() event.Category { return e.category }
func (e *testEvent) Topic() string            { return e.topic }
func (e *testEvent) Value() any               { return e.value }

func newInProcessEvent(topic string, value any) event.Event {
	return &testEvent{
		BaseInProcessEvent: &event.BaseInProcessEvent{},
		category:           event.CategoryInProcess,
		topic:              topic,
		value:              value,
	}
}

func TestInProcessBus_PublishDeliversToEverySubscriber(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	const handlers = 2
	done := make(chan int, handlers)
	for i := range handlers {
		if err := bus.Subscribe("topic-a", func(_ context.Context, _ event.Event) error {
			done <- i
			return nil
		}); err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
	}

	evt := newInProcessEvent("topic-a", 42)
	if err := bus.Publish(context.Background(), evt); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	// Delivery is asynchronous: every subscriber must eventually run.
	seen := map[int]bool{}
	for range handlers {
		select {
		case n := <-done:
			seen[n] = true
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for handler delivery")
		}
	}
	if len(seen) != handlers {
		t.Fatalf("expected both handlers to run, saw %v", seen)
	}
}

func TestInProcessBus_HandlerReceivesEventAndContext(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "v")

	got := make(chan event.Event, 1)
	if err := bus.Subscribe("topic-a", func(_ context.Context, evt event.Event) error {
		got <- evt
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	evt := newInProcessEvent("topic-a", "payload")
	if err := bus.Publish(ctx, evt); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if received := <-got; received != evt {
		t.Fatalf("handler received %v, want the published event", received)
	}
}

func TestInProcessBus_HandlerErrorIsNotReturned(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error {
		return context.DeadlineExceeded
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), newInProcessEvent("topic-a", nil)); err != nil {
		t.Fatalf("Publish must swallow handler errors, got %v", err)
	}
}

func TestInProcessBus_RejectsNonInProcessCategory(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	evt := &testEvent{
		BaseInProcessEvent: &event.BaseInProcessEvent{},
		category:           event.CategoryInterProcess,
		topic:              "topic-a",
	}
	if err := bus.Publish(context.Background(), evt); err == nil {
		t.Fatal("Publish must reject events whose category is not inprocess")
	}
}

func TestInProcessBus_NilHandlerRejected(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	if err := bus.Subscribe("topic-a", nil); err == nil {
		t.Fatal("Subscribe must reject a nil handler")
	}
}

func TestInProcessBus_OtherTopicsNotDelivered(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	called := false
	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error {
		called = true
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := bus.Publish(context.Background(), newInProcessEvent("topic-b", nil)); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if called {
		t.Fatal("handler subscribed to topic-a must not receive topic-b events")
	}
}

func TestInProcessBus_CloseTerminatesTheBus(t *testing.T) {
	bus := NewInProcessEventBus()

	fired := make(chan struct{})
	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error {
		close(fired)
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := bus.Publish(context.Background(), newInProcessEvent("topic-a", nil)); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	select {
	case <-fired:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not run")
	}

	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// After Close the bus is terminated: no further publishes or
	// subscribes are accepted.
	if err := bus.Publish(context.Background(), newInProcessEvent("topic-a", nil)); !errors.Is(err, exchange.ErrClosed) {
		t.Fatalf("Publish after Close must return ErrClosed, got %v", err)
	}
	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error { return nil }); !errors.Is(err, exchange.ErrClosed) {
		t.Fatalf("Subscribe after Close must return ErrClosed, got %v", err)
	}
	// Close is safe to call repeatedly.
	if err := bus.Close(context.Background()); err != nil {
		t.Fatalf("second Close must return nil, got %v", err)
	}
}

func TestInProcessBus_CloseWaitsForRunningHandlers(t *testing.T) {
	bus := NewInProcessEventBus()

	inFlight := make(chan struct{})
	release := make(chan struct{})
	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error {
		close(inFlight)
		<-release
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if err := bus.Publish(context.Background(), newInProcessEvent("topic-a", nil)); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	<-inFlight

	closed := make(chan error, 1)
	go func() { closed <- bus.Close(context.Background()) }()
	select {
	case err := <-closed:
		t.Fatalf("Close returned while a handler was still running: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-closed; err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestInProcessBus_PublishHandlerPanicIsRecovered(t *testing.T) {
	bus := NewInProcessEventBus()
	defer func() { _ = bus.Close(context.Background()) }()

	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error {
		panic("boom")
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	healthy := make(chan struct{})
	if err := bus.Subscribe("topic-a", func(context.Context, event.Event) error {
		close(healthy)
		return nil
	}); err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// The exchange-based implementation recovers handler panics so a faulty
	// handler cannot take down the publisher or the topic pool.
	if err := bus.Publish(context.Background(), newInProcessEvent("topic-a", nil)); err != nil {
		t.Fatalf("Publish after panicking handler: %v", err)
	}
	select {
	case <-healthy:
	case <-time.After(5 * time.Second):
		t.Fatal("healthy handler did not run alongside a panicking one")
	}
}
