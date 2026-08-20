package httpclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type mockRoundTripper struct {
	fn func(req *http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}

func TestRetryRoundTripper_Success(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusOK}, nil
		},
	})

	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryRoundTripper_RetryOn5xx(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
		},
	}, WithRetryBaseDelay(time.Millisecond))

	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls (1 initial + 2 retries), got %d", calls)
	}
}

func TestRetryRoundTripper_NoRetryOn4xx(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusBadRequest}, nil
		},
	})

	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryRoundTripper_RetryOnNetworkError(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return nil, &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection refused")}
		},
	}, WithRetryBaseDelay(time.Millisecond))

	_, err := rt.RoundTrip(&http.Request{})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryRoundTripper_NoRetryOnCanceled(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return nil, context.Canceled
		},
	})

	_, err := rt.RoundTrip(&http.Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryRoundTripper_NoRetryOnDeadlineExceeded(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return nil, context.DeadlineExceeded
		},
	})

	_, err := rt.RoundTrip(&http.Request{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryRoundTripper_BodyReplay(t *testing.T) {
	var calls int
	body := "request body"
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			b, _ := io.ReadAll(req.Body)
			req.Body.Close()
			if string(b) != body {
				t.Fatalf("expected body %q, got %q", body, string(b))
			}
			return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
		},
	}, WithRetryBaseDelay(time.Millisecond))

	req := &http.Request{
		Body: io.NopCloser(strings.NewReader(body)),
		GetBody: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(body)), nil
		},
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}

func TestRetryRoundTripper_ContextCancelDuringBackoff(t *testing.T) {
	rt := NewRetryRoundTripper(
		5,
		&mockRoundTripper{
			fn: func(req *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
			},
		},
		WithRetryBaseDelay(1*time.Hour),
		WithRetryMaxDelay(1*time.Hour),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &http.Request{}
	req = req.WithContext(ctx)

	_, err := rt.RoundTrip(req)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestRetryRoundTripper_ZeroMaxRetries(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(0, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
		},
	})

	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp.StatusCode)
	}
	if calls != 1 {
		t.Fatalf("expected 1 call, got %d", calls)
	}
}

func TestRetryRoundTripper_WithOptions(t *testing.T) {
	rt := NewRetryRoundTripper(2, nil,
		WithRetryBaseDelay(200*time.Millisecond),
		WithRetryMaxDelay(5*time.Second),
	)
	if rt.baseDelay != 200*time.Millisecond {
		t.Fatalf("expected baseDelay 200ms, got %v", rt.baseDelay)
	}
	if rt.maxDelay != 5*time.Second {
		t.Fatalf("expected maxDelay 5s, got %v", rt.maxDelay)
	}
}

func TestRetryRoundTripper_DefaultBaseDelay(t *testing.T) {
	rt := NewRetryRoundTripper(0, nil)
	if rt.baseDelay != 500*time.Millisecond {
		t.Fatalf("expected default baseDelay 500ms, got %v", rt.baseDelay)
	}
}

func TestRetryRoundTripper_ClosesBodyBeforeRetry(t *testing.T) {
	var calls int
	var closedAt []int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			body := &closeTrackingBody{Reader: strings.NewReader("error body")}
			closedAt = append(closedAt, 0)
			body.onClose = func() {
				closedAt[len(closedAt)-1] = calls
			}
			return &http.Response{StatusCode: http.StatusServiceUnavailable, Body: body}, nil
		},
	}, WithRetryBaseDelay(time.Millisecond))

	_, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
	// 前两次的失败响应 body 必须在各自调用结束后被关闭
	for i := 0; i < 2; i++ {
		if closedAt[i] != i+1 {
			t.Fatalf("body of attempt %d closed at call %d, want %d", i+1, closedAt[i], i+1)
		}
	}
}

type closeTrackingBody struct {
	io.Reader
	onClose func()
	closed  bool
}

func (b *closeTrackingBody) Close() error {
	if !b.closed {
		b.closed = true
		if b.onClose != nil {
			b.onClose()
		}
	}
	return nil
}

func TestRetryRoundTripper_RetryThenSuccess(t *testing.T) {
	var calls int
	rt := NewRetryRoundTripper(2, &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			calls++
			if calls < 3 {
				return &http.Response{StatusCode: http.StatusServiceUnavailable}, nil
			}
			return &http.Response{StatusCode: http.StatusOK}, nil
		},
	}, WithRetryBaseDelay(time.Millisecond))

	resp, err := rt.RoundTrip(&http.Request{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if calls != 3 {
		t.Fatalf("expected 3 calls, got %d", calls)
	}
}
