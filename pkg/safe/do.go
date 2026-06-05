package safe

import (
	"context"
	"log/slog"
	"runtime/debug"

	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func Do(ctx context.Context, f func() error) func() error {
	return func() (err error) {
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
}
