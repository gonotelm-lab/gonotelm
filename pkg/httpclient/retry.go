package httpclient

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/semconv/v1.41.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

type retryAttemptKey struct{}

func withRetryAttempt(ctx context.Context, attempt int) context.Context {
	return context.WithValue(ctx, retryAttemptKey{}, attempt)
}

func getRetryAttempt(ctx context.Context) int {
	a, _ := ctx.Value(retryAttemptKey{}).(int)
	return a
}

// 给每次重试的span attribute都打上标记
func retryAttemptInjectRoundTrip(next http.RoundTripper) http.RoundTripper {
	return RoundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// 如果是重试
		if cnt := getRetryAttempt(req.Context()); cnt > 0 {
			span := oteltrace.SpanFromContext(req.Context())
			// See: https://opentelemetry.io/docs/specs/semconv/http/http-spans/#http-request-retries-and-redirects
			span.SetAttributes(semconv.HTTPRequestResendCountKey.Int(cnt))
		}

		return next.RoundTrip(req)
	})
}

type RetryRoundTripper struct {
	maxRetries int
	baseDelay  time.Duration
	maxDelay   time.Duration
	next       http.RoundTripper
}

var _ http.RoundTripper = (*RetryRoundTripper)(nil)

type RetryOption func(*RetryRoundTripper)

func WithRetryBaseDelay(d time.Duration) RetryOption {
	return func(r *RetryRoundTripper) {
		r.baseDelay = d
	}
}

func WithRetryMaxDelay(d time.Duration) RetryOption {
	return func(r *RetryRoundTripper) {
		r.maxDelay = d
	}
}

func NewRetryRoundTripper(maxRetries int, next http.RoundTripper, opts ...RetryOption) *RetryRoundTripper {
	r := &RetryRoundTripper{
		maxRetries: maxRetries,
		baseDelay:  500 * time.Millisecond,
		maxDelay:   10 * time.Second,
		next:       next,
	}
	for _, opt := range opts {
		opt(r)
	}

	return r
}

func (r *RetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var (
		resp    *http.Response
		err     error
		attempt int
	)

	for {
		if attempt > 0 {
			// 已经是重试进来了 链路上打上重试 attribute
			req = req.WithContext(withRetryAttempt(req.Context(), attempt))
		}
		resp, err = r.next.RoundTrip(req)
		attempt++
		if err == nil && !r.shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}

		if attempt > r.maxRetries || !r.shouldRetry(err, resp) {
			return resp, err
		}

		// 重试前必须 drain 并关闭失败响应的 body：
		// 1. 连接才能复用，否则每次重试都新建连接
		// 2. otelhttp 等包装层依赖 body 关闭/读完来结束 span，
		//    不关闭会导致 span 永不结束
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		delay := r.computeBackoff(attempt)
		r.logRetry(req, attempt, delay, err, resp)

		if err := r.backoffSleep(req.Context(), delay); err != nil {
			return nil, err
		}

		if req.Body != nil && req.GetBody != nil {
			body, bodyErr := req.GetBody()
			if bodyErr != nil {
				return nil, bodyErr
			}
			req.Body = body
		}
	}
}

func (r *RetryRoundTripper) logRetry(
	req *http.Request,
	attempt int,
	delay time.Duration,
	err error,
	resp *http.Response,
) {
	attrs := []any{
		slog.Int("attempt", attempt),
		slog.Int("max_retries", r.maxRetries),
		slog.Duration("backoff", delay),
	}
	if req != nil {
		attrs = append(attrs, slog.String("method", req.Method))
		if req.URL != nil {
			attrs = append(attrs, slog.String("url", req.URL.String()))
		}
	}
	if err != nil {
		attrs = append(attrs, slog.Any("err", err))
	}
	if resp != nil {
		attrs = append(attrs, slog.Int("status", resp.StatusCode))
	}

	ctx := context.Background()
	if req != nil {
		ctx = req.Context()
	}
	slog.WarnContext(ctx, "http client retrying request", attrs...)
}

func (r *RetryRoundTripper) shouldRetryStatus(code int) bool {
	return code == 0 || code >= http.StatusInternalServerError
}

func (r *RetryRoundTripper) shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) { // ctx此时已经Done了，同一个ctx重试无意义
			return false
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return true
		}
		return true
	}

	if resp != nil && (resp.StatusCode >= http.StatusInternalServerError ||
		resp.StatusCode == http.StatusTooManyRequests ||
		resp.StatusCode == http.StatusRequestTimeout) {
		return true
	}

	return false
}

func (r *RetryRoundTripper) computeBackoff(attempt int) time.Duration {
	delay := min(r.baseDelay*(1<<(attempt-1)), r.maxDelay)
	if delay <= 0 {
		return 0
	}
	half := delay / 2
	if half <= 0 {
		return delay
	}
	jitter := time.Duration(rand.Int63n(int64(half)))
	return half + jitter
}

func (r *RetryRoundTripper) backoffSleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
