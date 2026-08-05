package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"

	pkgerr "github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/cloudwego/hertz/pkg/app"
)

func Recovery() app.HandlerFunc {
	return func(ctx context.Context, rc *app.RequestContext) {
		defer func() {
			if err := recover(); err != nil {
				stacks := debug.Stack()
				// log error
				slog.ErrorContext(
					ctx,
					"[panic] recover",
					slog.Any("err", err),
					slog.String("stacks", string(stacks)),
				)

				rc.AbortWithStatusJSON(http.StatusInternalServerError, pkgerr.ErrInner)
			}
		}()
		rc.Next(ctx)
	}
}
