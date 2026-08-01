package safe

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type ctxKey string

func TestMergeCancelCtx_KeepsCtx1Values(t *testing.T) {
	const key ctxKey = "k"
	ctx1 := context.WithValue(context.Background(), key, "from-ctx1")
	ctx2 := context.WithValue(context.Background(), key, "from-ctx2")

	merged, cancel := mergeCancelCtx(ctx1, ctx2)
	defer cancel()

	if got := merged.Value(key); got != "from-ctx1" {
		t.Fatalf("merged.Value = %v, want %q (ctx1 value)", got, "from-ctx1")
	}
}

func TestMergeCancelCtx_CancelWhenCtx1Canceled(t *testing.T) {
	ctx1, cancel1 := context.WithCancelCause(context.Background())
	ctx2 := context.Background()

	merged, cancel := mergeCancelCtx(ctx1, ctx2)
	defer cancel()

	cause := errors.New("ctx1 canceled")
	cancel1(cause)

	waitCanceled(t, merged)
	if got := context.Cause(merged); !errors.Is(got, cause) {
		t.Fatalf("context.Cause(merged) = %v, want %v", got, cause)
	}
}

func TestMergeCancelCtx_CancelWhenCtx2Canceled(t *testing.T) {
	ctx1 := context.Background()
	ctx2, cancel2 := context.WithCancelCause(context.Background())

	merged, cancel := mergeCancelCtx(ctx1, ctx2)
	defer cancel()

	cause := errors.New("ctx2 canceled")
	cancel2(cause)

	waitCanceled(t, merged)
	if got := context.Cause(merged); !errors.Is(got, cause) {
		t.Fatalf("context.Cause(merged) = %v, want %v", got, cause)
	}
}

func TestMergeCancelCtx_ManualCancel(t *testing.T) {
	merged, cancel := mergeCancelCtx(context.Background(), context.Background())

	cancel()

	waitCanceled(t, merged)
	if got := context.Cause(merged); !errors.Is(got, context.Canceled) {
		t.Fatalf("context.Cause(merged) = %v, want %v", got, context.Canceled)
	}
}

// ctx1 带 value 且 WithoutCancel（原 cancel 不影响），ctx2 负责取消。
func TestMergeCancelCtx_WithoutCancelCtx1_CancelByCtx2(t *testing.T) {
	const key ctxKey = "k"
	parent1, cancelParent1 := context.WithCancelCause(context.Background())
	parent1 = context.WithValue(parent1, key, "from-ctx1")
	ctx1 := context.WithoutCancel(parent1)

	ctx2, cancel2 := context.WithCancelCause(context.Background())

	merged, cancel := mergeCancelCtx(ctx1, ctx2)
	defer cancel()

	if got := merged.Value(key); got != "from-ctx1" {
		t.Fatalf("merged.Value = %v, want %q (ctx1 value)", got, "from-ctx1")
	}

	// WithoutCancel：取消 parent1 不应取消 merged
	cancelParent1(errors.New("parent1 canceled"))
	select {
	case <-merged.Done():
		t.Fatal("merged should not cancel when WithoutCancel parent is canceled")
	case <-time.After(50 * time.Millisecond):
	}

	cause := errors.New("ctx2 canceled")
	cancel2(cause)

	waitCanceled(t, merged)
	if got := context.Cause(merged); !errors.Is(got, cause) {
		t.Fatalf("context.Cause(merged) = %v, want %v", got, cause)
	}
	if got := merged.Value(key); got != "from-ctx1" {
		t.Fatalf("after cancel, merged.Value = %v, want %q", got, "from-ctx1")
	}
}

func TestDetachGo_KeepsValue_IgnoresWithoutCancelParent_CancelsByRoot(t *testing.T) {
	const key ctxKey = "k"
	reqCtx, cancelReq := context.WithCancelCause(context.Background())
	reqCtx = context.WithValue(reqCtx, key, "req-value")
	rootCtx, cancelRoot := context.WithCancelCause(context.Background())
	defer cancelRoot(nil)

	started := make(chan context.Context, 1)
	done := make(chan struct{})

	DetachGo(reqCtx, rootCtx, func(ctx context.Context) {
		started <- ctx
		<-ctx.Done()
		close(done)
	})

	detached := recvCtx(t, started)
	if got := detached.Value(key); got != "req-value" {
		t.Fatalf("detached.Value = %v, want %q", got, "req-value")
	}

	// 取消请求 ctx：任务应继续，detached 不 Done
	cancelReq(errors.New("request canceled"))
	select {
	case <-detached.Done():
		t.Fatal("detached should not cancel when request ctx is canceled")
	case <-done:
		t.Fatal("fn should keep running after request ctx cancel")
	case <-time.After(50 * time.Millisecond):
	}

	cause := errors.New("root canceled")
	cancelRoot(cause)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("fn did not observe root cancel")
	}
	if got := context.Cause(detached); !errors.Is(got, cause) {
		t.Fatalf("context.Cause(detached) = %v, want %v", got, cause)
	}
}

func TestDetachGo2_KeepsValue_IgnoresWithoutCancelParent_CancelsByRoot(t *testing.T) {
	const key ctxKey = "k"
	reqCtx, cancelReq := context.WithCancelCause(context.Background())
	reqCtx = context.WithValue(reqCtx, key, "req-value")
	rootCtx, cancelRoot := context.WithCancelCause(context.Background())
	defer cancelRoot(nil)

	started := make(chan context.Context, 1)
	var wg sync.WaitGroup

	DetachGo2(reqCtx, rootCtx, &wg, func(ctx context.Context) {
		started <- ctx
		<-ctx.Done()
	})

	detached := recvCtx(t, started)
	if got := detached.Value(key); got != "req-value" {
		t.Fatalf("detached.Value = %v, want %q", got, "req-value")
	}

	cancelReq(errors.New("request canceled"))
	select {
	case <-detached.Done():
		t.Fatal("detached should not cancel when request ctx is canceled")
	case <-time.After(50 * time.Millisecond):
	}

	cause := errors.New("root canceled")
	cancelRoot(cause)

	waitDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("DetachGo2 WaitGroup did not finish after root cancel")
	}
	if got := context.Cause(detached); !errors.Is(got, cause) {
		t.Fatalf("context.Cause(detached) = %v, want %v", got, cause)
	}
}

func recvCtx(t *testing.T, ch <-chan context.Context) context.Context {
	t.Helper()
	select {
	case ctx := <-ch:
		return ctx
	case <-time.After(time.Second):
		t.Fatal("fn was not started")
		return nil
	}
}

func waitCanceled(t *testing.T, ctx context.Context) {
	t.Helper()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("merged context was not canceled")
	}
}
