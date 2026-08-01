package httpclient

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"net/url"
	"time"
)

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
		baseDelay:  1 * time.Millisecond,
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
		resp, err = r.next.RoundTrip(req)
		attempt++
		if err == nil && !r.shouldRetryStatus(resp.StatusCode) {
			return resp, nil
		}

		if attempt > r.maxRetries || !r.shouldRetry(err, resp) {
			return resp, err
		}

		if err := r.backoffSleep(req.Context(), attempt); err != nil {
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

func (r *RetryRoundTripper) shouldRetryStatus(code int) bool {
	return code == 0 || code >= 500
}

func (r *RetryRoundTripper) shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return false
		}
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return true
		}
		return true
	}

	if resp != nil && resp.StatusCode >= 500 {
		return true
	}

	return false
}

func (r *RetryRoundTripper) backoffSleep(ctx context.Context, attempt int) error {
	delay := r.baseDelay * (1 << (attempt - 1))
	if delay > r.maxDelay {
		delay = r.maxDelay
	}

	jitter := time.Duration(rand.Int63n(int64(delay) / 2))
	delay = delay/2 + jitter

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
