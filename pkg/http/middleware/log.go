package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	pkghttp "github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
)

type logRequestOption struct {
	logAllError   bool
	slowThreshold time.Duration
}

type LogRequestOption func(*logRequestOption)

func WithLogAllError(logAllError bool) LogRequestOption {
	return func(o *logRequestOption) {
		o.logAllError = logAllError
	}
}

func WithSlowThreshold(slowThreshold time.Duration) LogRequestOption {
	return func(o *logRequestOption) {
		o.slowThreshold = slowThreshold
	}
}

func defaultLogRequestOption() *logRequestOption {
	return &logRequestOption{
		logAllError:   false,
		slowThreshold: 500 * time.Millisecond,
	}
}

func LogRequest(opts ...LogRequestOption) app.HandlerFunc {
	o := defaultLogRequestOption()
	for _, opt := range opts {
		opt(o)
	}

	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		c.Next(ctx)
		elapsed := time.Since(start)

		var (
			status = c.Response.StatusCode()
			method = string(c.Method())
			path   = string(c.Request.RequestURI())
		)

		if err, exist := c.Get(pkghttp.RequestContextRawErrKey); exist && err != nil {
			var (
				logFn  = slog.ErrorContext
				errMsg string
			)

			basicMsg := fmt.Sprintf("[HTTP][%d](%dms) %s:%s", status, elapsed.Milliseconds(), method, path)
			_, ok := c.Get(pkghttp.RequestContextInnerErrKey)
			errMsg = fmt.Sprintf("%+v", err)
			if ok {
				if status < http.StatusInternalServerError {
					logFn = slog.WarnContext
				}
			}

			if status >= http.StatusBadRequest || o.logAllError {
				logFn(ctx, basicMsg, "err", errMsg)
			}
		} else {
			skip := c.GetBool(pkghttp.RequestContextSlowLogSkip)
			duration := o.slowThreshold
			if d := c.GetDuration(pkghttp.RequestContextSlowLogThreshold); d > 0 {
				duration = d
			}

			// not error check slow request
			if elapsed > duration && !skip {
				basicMsg := fmt.Sprintf("[SLOW HTTP][%d](%dms) %s:%s", status, elapsed.Milliseconds(), method, path)
				slog.WarnContext(ctx, basicMsg)
			}
		}
	}
}
