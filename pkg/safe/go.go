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

func DetachGo(ctx context.Context, fn func()) {
	ctx = context.WithoutCancel(ctx)
	Go(ctx, fn)
}

func DetachGo2(ctx context.Context, wg *sync.WaitGroup, fn func()) {
	ctx = context.WithoutCancel(ctx)
	Go2(ctx, wg, fn)
}
