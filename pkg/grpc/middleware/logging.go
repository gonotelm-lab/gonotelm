package middleware

import (
	"context"
	"encoding/json"
	stderr "errors"
	"fmt"
	"log/slog"
	"time"

	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type loggingOption struct {
	logAllError   bool
	slowThreshold time.Duration
}

type LoggingOption func(*loggingOption)

func WithLogAllError(logAllError bool) LoggingOption {
	return func(o *loggingOption) {
		o.logAllError = logAllError
	}
}

func WithSlowThreshold(slowThreshold time.Duration) LoggingOption {
	return func(o *loggingOption) {
		o.slowThreshold = slowThreshold
	}
}

func defaultLoggingOption() *loggingOption {
	return &loggingOption{
		logAllError:   false,
		slowThreshold: 500 * time.Millisecond,
	}
}

// UnaryLogging 行为对齐 HTTP Logging：错误按 InnerError / gRPC status 分级；
// 成功则按耗时打 Debug / SLOW Warn。
// 需放在 UnaryError 内侧，才能直接拿到业务 *InnerError。
func UnaryLogging(opts ...LoggingOption) grpc.UnaryServerInterceptor {
	o := defaultLoggingOption()
	for _, opt := range opts {
		opt(o)
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		elapsed := time.Since(start)
		method := info.FullMethod

		if err != nil {
			logRPCError(ctx, err, method, elapsed, o)
			return resp, err
		}

		if elapsed > o.slowThreshold {
			slog.WarnContext(ctx, fmt.Sprintf("[SLOW gRPC][OK](%dms) %s", elapsed.Milliseconds(), method))
		} else {
			slog.DebugContext(ctx, fmt.Sprintf("[gRPC][OK](%dms) %s", elapsed.Milliseconds(), method))
		}

		return resp, nil
	}
}

func logRPCError(
	ctx context.Context,
	err error,
	method string,
	elapsed time.Duration,
	o *loggingOption,
) {
	code, ie := classifyRPCError(err)
	basicMsg := fmt.Sprintf("[gRPC][%s](%dms) %s", code.String(), elapsed.Milliseconds(), method)
	errMsg := fmt.Sprintf("%+v", err)

	logFn := slog.ErrorContext
	if ie != nil && !ie.ShouldLogError() {
		logFn = slog.WarnContext
	} else if ie == nil && code != codes.Internal && code != codes.Unknown {
		logFn = slog.WarnContext
	}

	// 对齐 HTTP：非严重失败默认也记；Internal 一律记；logAllError 时全记
	if o.logAllError || code != codes.OK {
		logFn(ctx, basicMsg, "err", errMsg)
	}
}

func classifyRPCError(err error) (codes.Code, *xerror.InnerError) {
	cause := xerror.Cause(err)

	var ie *xerror.InnerError
	if stderr.As(cause, &ie) {
		if ie.ShouldLogError() {
			return codes.Internal, ie
		}
		return codes.InvalidArgument, ie
	}

	st, ok := status.FromError(err)
	if !ok {
		return codes.Unknown, nil
	}

	if parsed := parseInnerErrorJSON(st.Message()); parsed != nil {
		return st.Code(), parsed
	}

	return st.Code(), nil
}

func parseInnerErrorJSON(msg string) *xerror.InnerError {
	if msg == "" {
		return nil
	}

	var ie xerror.InnerError
	if err := json.Unmarshal([]byte(msg), &ie); err != nil {
		return nil
	}
	if ie.Code == 0 && ie.Message == "" {
		return nil
	}
	return &ie
}
