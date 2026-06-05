package safe

import (
	"context"
	"log/slog"
	"runtime/debug"
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
