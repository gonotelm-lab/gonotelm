package safe

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/gonotelm-lab/gonotelm/pkg/trace"

	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// spanStarter starts a span inside a goroutine.
type spanStarter func(ctx context.Context, spanName string) (context.Context, oteltrace.Span)

// goRun starts fn in a goroutine with its own span.
func goRun(ctx context.Context, spanName string, start spanStarter, fn func(context.Context)) {
	go func() {
		ctx, span := start(ctx, spanName)
		defer span.End()

		defer func() {
			if e := recover(); e != nil {
				span.RecordError(fmt.Errorf("%v", e))
				span.SetStatus(codes.Error, "panic")
				slog.ErrorContext(ctx, "safe goroutine panic",
					slog.Any("err", e),
					slog.String("stacks", string(debug.Stack())),
				)
			}
		}()

		fn(ctx)
	}()
}

// startParentSpan starts a span parented to the span in ctx.
// Used when the caller waits for the goroutine (Go2/DetachGo2 with a WaitGroup),
// so the parent span is still open when the child ends.
func startParentSpan(ctx context.Context, spanName string) (context.Context, oteltrace.Span) {
	return trace.GetOtelTracer().Start(ctx, spanName)
}

// startLinkedSpan starts a root span that links to the span in ctx.
// The caller's span is only linked, never parented, because fire-and-forget
// goroutines (Go/DetachGo) may outlive the caller's span.
func startLinkedSpan(ctx context.Context, spanName string) (context.Context, oteltrace.Span) {
	var links []oteltrace.Link
	if sc := oteltrace.SpanContextFromContext(ctx); sc.IsValid() {
		links = append(links, oteltrace.Link{SpanContext: sc})
	}
	return trace.GetOtelTracer().Start(ctx, spanName,
		oteltrace.WithNewRoot(),
		oteltrace.WithLinks(links...),
	)
}

func Go(ctx context.Context, spanName string, fn func(context.Context)) {
	goRun(ctx, spanName, startLinkedSpan, fn)
}

func Go2(ctx context.Context, spanName string, wg *sync.WaitGroup, fn func(context.Context)) {
	wg.Add(1)
	goRun(ctx, spanName, startParentSpan, func(ctx context.Context) {
		defer wg.Done()
		fn(ctx)
	})
}

// DetachGo runs fn detached from withoutCancelCtx's cancellation, but still
// under rootCtx's control. Fire-and-forget: the detached work gets its own
// root span linked to the caller's span.
func DetachGo(
	withoutCancelCtx context.Context,
	rootCtx context.Context,
	spanName string,
	fn func(ctx context.Context),
) {
	withoutCancelCtx = context.WithoutCancel(withoutCancelCtx)
	ctx, cancel := mergeCancelCtx(withoutCancelCtx, rootCtx)
	goRun(ctx, spanName, startLinkedSpan, func(ctx context.Context) {
		fn(ctx)
		cancel()
	})
}

// DetachGo2 is DetachGo with a WaitGroup. The caller waits for the goroutine,
// so the child span is parented to the caller's span.
func DetachGo2(
	withoutCancelCtx context.Context,
	rootCtx context.Context,
	spanName string,
	wg *sync.WaitGroup,
	fn func(ctx context.Context),
) {
	withoutCancelCtx = context.WithoutCancel(withoutCancelCtx)
	ctx, cancel := mergeCancelCtx(withoutCancelCtx, rootCtx)
	Go2(ctx, spanName, wg, func(ctx context.Context) {
		fn(ctx)
		cancel()
	})
}

// 合并两个ctx 返回新的ctx 和 cancelFunc
//
// 新的ctx包含ctx1的value 并且新的ctx会在ctx1或者ctx2取消时取消
func mergeCancelCtx(ctx1, ctx2 context.Context) (context.Context, context.CancelFunc) {
	ctx1, cancel := context.WithCancelCause(ctx1)
	if ctx2 == nil {
		return ctx1, func() {
			cancel(context.Canceled)
		}
	}

	stop := context.AfterFunc(ctx2, func() {
		cancel(context.Cause(ctx2))
	})

	return ctx1, func() {
		stop()
		cancel(context.Canceled)
	}
}
