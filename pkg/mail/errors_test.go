package mail

import (
	"context"
	stderr "errors"
	"testing"
	"time"
)

func TestIsTransient(t *testing.T) {
	cause := stderr.New("boom")

	if IsTransient(nil) {
		t.Fatal("nil must not be transient")
	}
	if !IsTransient(Transient(cause)) {
		t.Fatal("Transient must be transient")
	}
	if IsTransient(Permanent(cause)) {
		t.Fatal("Permanent must not be transient")
	}
	if !stderr.Is(Transient(cause), cause) {
		t.Fatal("classification must keep the cause")
	}
	if !IsTransient(context.DeadlineExceeded) {
		t.Fatal("deadline exceeded is retryable")
	}
	if IsTransient(context.Canceled) {
		t.Fatal("caller cancellation is not retryable")
	}
	if !IsTransient(&RateLimitError{Message: "slow down", RetryAfter: 3 * time.Second}) {
		t.Fatal("rate limit is transient")
	}
}

func TestIsPermanent(t *testing.T) {
	cause := stderr.New("boom")

	if !IsPermanent(Permanent(cause)) {
		t.Fatal("Permanent must be permanent")
	}
	if IsPermanent(Transient(cause)) {
		t.Fatal("Transient must not be permanent")
	}
	if IsPermanent(nil) {
		t.Fatal("nil must not be permanent")
	}
}

func TestRateLimitErrorMessage(t *testing.T) {
	withWait := (&RateLimitError{Message: "slow down", RetryAfter: 3 * time.Second}).Error()
	if withWait != "mail: rate limited, retry after 3s: slow down" {
		t.Fatalf("unexpected message: %q", withWait)
	}

	withoutWait := (&RateLimitError{Message: "slow down"}).Error()
	if withoutWait != "mail: rate limited: slow down" {
		t.Fatalf("unexpected message: %q", withoutWait)
	}
}
