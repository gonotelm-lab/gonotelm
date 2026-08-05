package safe

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func setupTracer(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(sdktrace.NewTracerProvider())
	})
	return exporter
}

func waitSpan(t *testing.T, exporter *tracetest.InMemoryExporter, name string) tracetest.SpanStub {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		for _, s := range exporter.GetSpans() {
			if s.Name == name {
				return s
			}
		}
		select {
		case <-deadline:
			t.Fatalf("span %q not exported", name)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func TestGoCreatesLinkedSpan(t *testing.T) {
	exporter := setupTracer(t)

	// 父 span:模拟请求 span
	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent")
	parentSpan.End()

	started := make(chan struct{})
	Go(parentCtx, "child", func(ctx context.Context) {
		defer close(started)
		// fn 拿到的 ctx 应携带新 span
		sc := oteltrace.SpanContextFromContext(ctx)
		if !sc.IsValid() {
			t.Errorf("fn ctx has no span")
		}
	})
	<-started

	child := waitSpan(t, exporter, "child")
	parent := waitSpan(t, exporter, "parent")

	// child 不应以 parent 为父(Link 关系)
	if child.Parent.SpanID() == parent.SpanContext.SpanID() {
		t.Errorf("child span should not be parented to parent span")
	}
	// child 应有 1 个 link 指向 parent
	if len(child.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(child.Links))
	}
	if child.Links[0].SpanContext.TraceID() != parent.SpanContext.TraceID() {
		t.Errorf("link trace id mismatch")
	}
}

func TestDetachGoCreatesLinkedSpan(t *testing.T) {
	exporter := setupTracer(t)

	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "detach-parent")
	parentSpan.End()

	rootCtx := context.Background()
	started := make(chan struct{})
	DetachGo(parentCtx, rootCtx, "detached-child", func(ctx context.Context) {
		defer close(started)
	})
	<-started

	child := waitSpan(t, exporter, "detached-child")
	if len(child.Links) != 1 {
		t.Fatalf("expected 1 link, got %d", len(child.Links))
	}
}

func TestGo2SpanWithWaitGroupIsParented(t *testing.T) {
	exporter := setupTracer(t)

	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "go2-parent")
	parentSpan.End()

	var wg sync.WaitGroup
	Go2(parentCtx, "wg-child", &wg, func(ctx context.Context) {})
	wg.Wait()

	child := waitSpan(t, exporter, "wg-child")
	parent := waitSpan(t, exporter, "go2-parent")
	// 有 wg 参数:调用方等待,使用 parent 关系
	if child.Parent.SpanID() != parent.SpanContext.SpanID() {
		t.Errorf("child span should be parented to parent span, got %s want %s",
			child.Parent.SpanID(), parent.SpanContext.SpanID())
	}
	if len(child.Links) != 0 {
		t.Errorf("parented child should have no links, got %d", len(child.Links))
	}
}

func TestGo2SpanWithoutParent(t *testing.T) {
	exporter := setupTracer(t)

	var wg sync.WaitGroup
	Go2(context.Background(), "wg-child", &wg, func(ctx context.Context) {})
	wg.Wait()

	child := waitSpan(t, exporter, "wg-child")
	if len(child.Links) != 0 {
		t.Errorf("no parent span, expected 0 links, got %d", len(child.Links))
	}
}

func TestDetachGo2CreatesParentSpan(t *testing.T) {
	exporter := setupTracer(t)

	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "detach2-parent")
	parentSpan.End()

	rootCtx := context.Background()
	var wg sync.WaitGroup
	DetachGo2(parentCtx, rootCtx, "detach2-child", &wg, func(ctx context.Context) {})
	wg.Wait()

	child := waitSpan(t, exporter, "detach2-child")
	parent := waitSpan(t, exporter, "detach2-parent")
	if child.Parent.SpanID() != parent.SpanContext.SpanID() {
		t.Errorf("DetachGo2 child should be parented, got %s want %s",
			child.Parent.SpanID(), parent.SpanContext.SpanID())
	}
}

func TestGoSpanPanicSetsErrorStatus(t *testing.T) {
	exporter := setupTracer(t)

	Go(context.Background(), "panic-child", func(ctx context.Context) {
		panic("boom")
	})

	child := waitSpan(t, exporter, "panic-child")
	if child.Status.Code != codes.Error {
		t.Errorf("status: got %v want %v", child.Status.Code, codes.Error)
	}
}
