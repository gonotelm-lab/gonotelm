package safe

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// Do 执行 f，panic 会被兜底转成 errors.ErrInner。
func Do(ctx context.Context, f func() error) (err error) {
	defer func() {
		if e := recover(); e != nil {
			slog.ErrorContext(ctx, "safe do panic", slog.Any("err", e),
				slog.String("stacks", string(debug.Stack())),
			)

			err = errors.ErrInner
		}
	}()

	return f()
}

// DoWithContext 执行 f(ctx)，panic 会被兜底转成 errors.ErrInner。
func DoWithContext(ctx context.Context, f func(context.Context) error) (err error) {
	defer func() {
		if e := recover(); e != nil {
			slog.ErrorContext(ctx, "safe do with context panic", slog.Any("err", e),
				slog.String("stacks", string(debug.Stack())),
			)

			err = errors.ErrInner
		}
	}()

	return f(ctx)
}
