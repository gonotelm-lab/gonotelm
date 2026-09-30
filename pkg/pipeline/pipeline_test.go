package pipeline

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

// plainStep implements only Step.
type plainStep struct {
	name string
	exec func(ctx context.Context, data *Data) error
}

func (s *plainStep) Name() string { return s.name }

func (s *plainStep) Execute(ctx context.Context, data *Data) error {
	if s.exec == nil {
		return nil
	}
	return s.exec(ctx, data)
}

// conditionalStep implements Step + ConditionalStep.
type conditionalStep struct {
	plainStep
	should func(ctx context.Context, data *Data) bool
}

func (s *conditionalStep) ShouldExecute(ctx context.Context, data *Data) bool {
	if s.should == nil {
		return true
	}
	return s.should(ctx, data)
}

type recorder struct {
	mu    sync.Mutex
	names []string
}

func (r *recorder) add(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.names = append(r.names, name)
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.names...)
}

func recordStep(rec *recorder, name string) *plainStep {
	return &plainStep{name: name, exec: func(context.Context, *Data) error {
		rec.add(name)
		return nil
	}}
}

func TestPipeline_Name(t *testing.T) {
	if got := New("demo").Name(); got != "demo" {
		t.Fatalf("Name() = %q, want %q", got, "demo")
	}
}

func TestPipeline_ExecuteEmpty(t *testing.T) {
	if err := New("empty").Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("empty pipeline Execute = %v, want nil", err)
	}
}

func TestPipeline_AddStepAndAddSteps_RunInOrder(t *testing.T) {
	rec := &recorder{}
	p := New("order")
	p.AddStep(recordStep(rec, "a"))
	p.AddSteps(recordStep(rec, "b"), recordStep(rec, "c"))

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if got, want := rec.snapshot(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
		t.Fatalf("execution order = %v, want %v", got, want)
	}
}

func TestPipeline_SequentialError_StopsAndReturnsError(t *testing.T) {
	rec := &recorder{}
	p := New("error")
	p.AddStep(recordStep(rec, "a"))
	p.AddStep(&plainStep{name: "b", exec: func(context.Context, *Data) error {
		rec.add("b")
		return errBoom
	}})
	p.AddStep(recordStep(rec, "c"))

	err := p.Execute(context.Background(), NewData())
	if !errors.Is(err, errBoom) {
		t.Fatalf("Execute = %v, want %v", err, errBoom)
	}
	if got, want := rec.snapshot(), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Fatalf("execution order = %v, want %v (step after failure must not run)", got, want)
	}
}

func TestPipeline_SequentialSteps_ShareData(t *testing.T) {
	p := New("share")
	p.AddStep(&plainStep{name: "write", exec: func(_ context.Context, d *Data) error {
		d.Set("answer", 42)
		return nil
	}})
	p.AddStep(&plainStep{name: "read", exec: func(_ context.Context, d *Data) error {
		if got := d.GetInt("answer"); got != 42 {
			t.Fatalf("second step read %d, want 42", got)
		}
		return nil
	}})

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
}

func TestPipeline_ConditionalStep_SkipAndRun(t *testing.T) {
	rec := &recorder{}
	p := New("conditional")
	p.AddStep(&conditionalStep{
		plainStep: plainStep{name: "skip", exec: func(context.Context, *Data) error {
			rec.add("skip")
			return nil
		}},
		should: func(context.Context, *Data) bool { return false },
	})
	p.AddStep(&conditionalStep{
		plainStep: plainStep{name: "run", exec: func(context.Context, *Data) error {
			rec.add("run")
			return nil
		}},
		should: func(context.Context, *Data) bool { return true },
	})
	// nil should -> default to execute.
	p.AddStep(&conditionalStep{
		plainStep: plainStep{name: "default", exec: func(context.Context, *Data) error {
			rec.add("default")
			return nil
		}},
	})

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if got, want := rec.snapshot(), []string{"run", "default"}; !slices.Equal(got, want) {
		t.Fatalf("executed steps = %v, want %v", got, want)
	}
}

func TestPipeline_ConditionalStep_SeesDataWrittenEarlier(t *testing.T) {
	rec := &recorder{}
	p := New("conditional-data")
	p.AddStep(&plainStep{name: "seed", exec: func(_ context.Context, d *Data) error {
		rec.add("seed")
		d.Set("enabled", true)
		return nil
	}})
	p.AddStep(&conditionalStep{
		plainStep: plainStep{name: "gated", exec: func(context.Context, *Data) error {
			rec.add("gated")
			return nil
		}},
		should: func(_ context.Context, d *Data) bool { return d.GetBool("enabled") },
	})

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if got, want := rec.snapshot(), []string{"seed", "gated"}; !slices.Equal(got, want) {
		t.Fatalf("executed steps = %v, want %v", got, want)
	}
}

func TestNewStepFunc_RunsAndNamesStep(t *testing.T) {
	rec := &recorder{}
	step := NewStepFunc("fn", func(context.Context, *Data) error {
		rec.add("fn")
		return nil
	})

	if got := step.Name(); got != "fn" {
		t.Fatalf("Name() = %q, want %q", got, "fn")
	}
	if err := step.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if got, want := rec.snapshot(), []string{"fn"}; !slices.Equal(got, want) {
		t.Fatalf("executed = %v, want %v", got, want)
	}
}

func TestNewStepFunc_PropagatesError(t *testing.T) {
	step := NewStepFunc("fail", func(context.Context, *Data) error { return errBoom })
	if err := step.Execute(context.Background(), NewData()); !errors.Is(err, errBoom) {
		t.Fatalf("Execute = %v, want %v", err, errBoom)
	}
}

func TestNewConditionalStepFunc_SkipAndRun(t *testing.T) {
	rec := &recorder{}
	p := New("conditional-func")
	p.AddSteps(
		NewConditionalStepFunc("skip",
			func(context.Context, *Data) bool { return false },
			func(context.Context, *Data) error { rec.add("skip"); return nil },
		),
		NewConditionalStepFunc("run",
			func(context.Context, *Data) bool { return true },
			func(context.Context, *Data) error { rec.add("run"); return nil },
		),
	)

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if got, want := rec.snapshot(), []string{"run"}; !slices.Equal(got, want) {
		t.Fatalf("executed = %v, want %v", got, want)
	}
}

func TestNewConditionalStepFunc_NilShouldDefaultsToRun(t *testing.T) {
	var ran atomic.Bool
	step := NewConditionalStepFunc("default",
		nil,
		func(context.Context, *Data) error { ran.Store(true); return nil },
	)

	if err := step.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if !ran.Load() {
		t.Fatal("step with nil should did not run")
	}
}

func TestNewConditionalStepFunc_SeesDataWrittenEarlier(t *testing.T) {
	rec := &recorder{}
	p := New("conditional-func-data")
	p.AddStep(NewStepFunc("seed", func(_ context.Context, d *Data) error {
		rec.add("seed")
		d.Set("enabled", true)
		return nil
	}))
	p.AddStep(NewConditionalStepFunc("gated",
		func(_ context.Context, d *Data) bool { return d.GetBool("enabled") },
		func(context.Context, *Data) error { rec.add("gated"); return nil },
	))

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if got, want := rec.snapshot(), []string{"seed", "gated"}; !slices.Equal(got, want) {
		t.Fatalf("executed steps = %v, want %v", got, want)
	}
}

func TestNewConditionalStepFunc_WorksAsParallelChild(t *testing.T) {
	var ran atomic.Bool
	group := NewParallelStep("group", false,
		NewConditionalStepFunc("gated",
			func(context.Context, *Data) bool { return false },
			func(context.Context, *Data) error { ran.Store(true); return nil },
		),
	)

	if err := group.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if ran.Load() {
		t.Fatal("conditional child ran despite ShouldExecute returning false")
	}
}

func TestParallel_Name(t *testing.T) {
	if got := NewParallelStep("fetch-all", false).Name(); got != "fetch-all" {
		t.Fatalf("Name() = %q, want %q", got, "fetch-all")
	}
}

func TestParallel_Empty_ReturnsNil(t *testing.T) {
	if err := NewParallelStep("empty", true).Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("empty parallel Execute = %v, want nil", err)
	}
}

// The composite is an ordinary sequential step for the pipeline: it runs once,
// in position, and the pipeline continues only after the whole group is done.
func TestParallel_IsASequentialStep(t *testing.T) {
	rec := &recorder{}
	p := New("compose")
	p.AddStep(recordStep(rec, "before"))
	p.AddStep(NewParallelStep("group", false, recordStep(rec, "a"), recordStep(rec, "b")))
	p.AddStep(recordStep(rec, "after"))

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}

	got := rec.snapshot()
	if len(got) != 4 {
		t.Fatalf("executed = %v, want 4 steps", got)
	}
	if got[0] != "before" || got[3] != "after" {
		t.Fatalf("executed = %v, want before first and after last", got)
	}
	if !slices.Contains(got, "a") || !slices.Contains(got, "b") {
		t.Fatalf("executed = %v, want both a and b", got)
	}
}

func TestParallel_RunsChildrenConcurrently(t *testing.T) {
	// Both children must be in flight at the same time for the barrier to release.
	var barrier sync.WaitGroup
	barrier.Add(2)
	var completed atomic.Int32

	child := func(name string) *plainStep {
		return &plainStep{name: name, exec: func(context.Context, *Data) error {
			barrier.Done()
			barrier.Wait()
			completed.Add(1)
			return nil
		}}
	}

	group := NewParallelStep("group", false, child("c1"), child("c2"))
	done := make(chan error, 1)
	go func() { done <- group.Execute(context.Background(), NewData()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Execute = %v, want nil", err)
		}
		if got := completed.Load(); got != 2 {
			t.Fatalf("children completed = %d, want 2", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("children never ran concurrently (barrier was not released)")
	}
}

func TestParallel_AbortOnError_ReturnsFirstErrorAndCancelsSiblings(t *testing.T) {
	var canceled atomic.Bool

	failing := &plainStep{name: "fail", exec: func(context.Context, *Data) error {
		return errBoom
	}}
	blocking := &plainStep{name: "block", exec: func(ctx context.Context, _ *Data) error {
		select {
		case <-ctx.Done():
			canceled.Store(true)
			return ctx.Err()
		case <-time.After(2 * time.Second):
			return nil
		}
	}}

	group := NewParallelStep("group", true, failing, blocking)
	err := group.Execute(context.Background(), NewData())

	if !errors.Is(err, errBoom) {
		t.Fatalf("Execute = %v, want %v", err, errBoom)
	}
	if !canceled.Load() {
		t.Fatal("sibling was not cancelled after the first child error")
	}
}

func TestParallel_AbortOnError_AbortsPipeline(t *testing.T) {
	rec := &recorder{}
	p := New("abort")
	p.AddStep(NewParallelStep("group", true, &plainStep{name: "fail", exec: func(context.Context, *Data) error {
		return errBoom
	}}))
	p.AddStep(recordStep(rec, "after"))

	err := p.Execute(context.Background(), NewData())
	if !errors.Is(err, errBoom) {
		t.Fatalf("Execute = %v, want %v", err, errBoom)
	}
	if got := rec.snapshot(); len(got) != 0 {
		t.Fatalf("steps after the failed group ran: %v, want none", got)
	}
}

func TestParallel_NoAbortOnError_LogsAndContinues(t *testing.T) {
	var siblingRan atomic.Bool
	rec := &recorder{}
	p := New("no-abort")
	p.AddStep(NewParallelStep("group", false,
		&plainStep{name: "fail", exec: func(context.Context, *Data) error {
			return errBoom
		}},
		&plainStep{name: "ok", exec: func(context.Context, *Data) error {
			siblingRan.Store(true)
			return nil
		}},
	))
	p.AddStep(recordStep(rec, "after"))

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil (child errors must not fail the group)", err)
	}
	if !siblingRan.Load() {
		t.Fatal("sibling child did not run")
	}
	if got, want := rec.snapshot(), []string{"after"}; !slices.Equal(got, want) {
		t.Fatalf("pipeline continued with %v, want %v", got, want)
	}
}

func TestParallel_ChildPanicIsContained(t *testing.T) {
	rec := &recorder{}
	p := New("panic")
	p.AddStep(NewParallelStep("group", false,
		&plainStep{name: "panicky", exec: func(context.Context, *Data) error {
			panic("boom")
		}},
		recordStep(rec, "ok"),
	))
	p.AddStep(recordStep(rec, "after"))

	if err := p.Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil (panic must be contained)", err)
	}
	if got, want := rec.snapshot(), []string{"ok", "after"}; !slices.Equal(got, want) {
		t.Fatalf("executed = %v, want %v", got, want)
	}
}

func TestParallel_ConditionalChildSkipped(t *testing.T) {
	var ran atomic.Bool
	skipped := &conditionalStep{
		plainStep: plainStep{name: "gated", exec: func(context.Context, *Data) error {
			ran.Store(true)
			return nil
		}},
		should: func(context.Context, *Data) bool { return false },
	}

	if err := NewParallelStep("group", false, skipped).Execute(context.Background(), NewData()); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if ran.Load() {
		t.Fatal("conditional child ran despite ShouldExecute returning false")
	}
}

func TestParallel_ChildrenShareData(t *testing.T) {
	group := NewParallelStep("group", false,
		&plainStep{name: "a", exec: func(_ context.Context, d *Data) error { d.Set("a", 1); return nil }},
		&plainStep{name: "b", exec: func(_ context.Context, d *Data) error { d.Set("b", 2); return nil }},
		&plainStep{name: "c", exec: func(_ context.Context, d *Data) error { d.Set("c", 3); return nil }},
	)

	data := NewData()
	if err := group.Execute(context.Background(), data); err != nil {
		t.Fatalf("Execute = %v, want nil", err)
	}
	if data.GetInt("a") != 1 || data.GetInt("b") != 2 || data.GetInt("c") != 3 {
		t.Fatalf("data = a:%d b:%d c:%d, want 1 2 3", data.GetInt("a"), data.GetInt("b"), data.GetInt("c"))
	}
}
