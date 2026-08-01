package safe

import (
	"context"
	"log/slog"
	"runtime/debug"
	"sync"
)

func Go(ctx context.Context, fn func()) {
	go func() {
		defer func() {
			if e := recover(); e != nil {
				slog.ErrorContext(ctx, "safe go panic", slog.Any("err", e),
					slog.String("stacks", string(debug.Stack())),
				)
			}
		}()

		fn()
	}()
}

func Go2(ctx context.Context, wg *sync.WaitGroup, fn func()) {
	wg.Go(func() {
		defer func() {
			if e := recover(); e != nil {
				slog.ErrorContext(ctx, "safe go2 panic", slog.Any("err", e),
					slog.String("stacks", string(debug.Stack())),
				)
			}
		}()

		fn()
	})
}

// 从 withoutCancelCtx 中分离出新的ctx 令fn的执行不受withoutCancelCtx的控制
// 但fn的执行仍然在rootCtx的控制下
//
// 主要使用场景：执行一个不受 withoutCancelCtx 影响的独立任务 但是需要保证这个任务在 rootCtx 的控制下
func DetachGo(
	withoutCancelCtx context.Context,
	rootCtx context.Context,
	fn func(ctx context.Context),
) {
	withoutCancelCtx = context.WithoutCancel(withoutCancelCtx)
	ctx, cancel := mergeCancelCtx(withoutCancelCtx, rootCtx)
	Go(ctx, func() {
		fn(ctx)
		cancel()
	})
}

func DetachGo2(
	withoutCancelCtx context.Context,
	rootCtx context.Context,
	wg *sync.WaitGroup,
	fn func(ctx context.Context),
) {
	withoutCancelCtx = context.WithoutCancel(withoutCancelCtx)
	ctx, cancel := mergeCancelCtx(withoutCancelCtx, rootCtx)
	Go2(ctx, wg, func() {
		fn(ctx)
		cancel()
	})
}

// 合并两个ctx 返回新的ctx 和 cancelFunc
//
// 新的ctx包含ctx1的value 并且新的ctx会在ctx1或者ctx2取消时取消
func mergeCancelCtx(ctx1, ctx2 context.Context) (context.Context, context.CancelFunc) {
	ctx1, cancel := context.WithCancelCause(ctx1)
	stop := context.AfterFunc(ctx2, func() {
		cancel(context.Cause(ctx2))
	})

	return ctx1, func() {
		stop()
		cancel(context.Canceled)
	}
}
