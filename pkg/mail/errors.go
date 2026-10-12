package mail

import (
	"context"
	stderr "errors"
	"fmt"
	"net"
	"time"
)

// Failure kinds; callers use them to decide whether to retry.
var (
	// ErrTransient: upstream temporarily unavailable (rate limit, network, timeout, 5xx).
	ErrTransient = stderr.New("mail: transient failure")
	// ErrPermanent: this email cannot be delivered (bad address, unverified domain,
	// rejected); retrying will not help.
	ErrPermanent = stderr.New("mail: permanent failure")
)

// RateLimitError means the upstream throttled us. RetryAfter is the suggested wait,
// 0 when the upstream did not say.
type RateLimitError struct {
	Message    string
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("mail: rate limited, retry after %s: %s", e.RetryAfter, e.Message)
	}

	return "mail: rate limited: " + e.Message
}

// Is makes RateLimitError match ErrTransient.
func (e *RateLimitError) Is(target error) bool {
	return target == ErrTransient
}

// Transient marks err retryable, keeping the cause.
func Transient(err error) error {
	if err == nil {
		return nil
	}

	return &classified{kind: ErrTransient, err: err}
}

// Permanent marks err not retryable, keeping the cause.
func Permanent(err error) error {
	if err == nil {
		return nil
	}

	return &classified{kind: ErrPermanent, err: err}
}

// IsTransient reports whether a failure is worth retrying. A caller-initiated
// context.Canceled is not: we gave up ourselves. Unclassified network errors count
// as transient, so an implementation that forgets to classify retries once too often
// rather than never.
func IsTransient(err error) bool {
	if err == nil || stderr.Is(err, context.Canceled) {
		return false
	}
	if stderr.Is(err, ErrPermanent) {
		return false
	}
	if stderr.Is(err, ErrTransient) || stderr.Is(err, context.DeadlineExceeded) {
		return true
	}

	if _, ok := stderr.AsType[net.Error](err); ok {
		return true
	}

	return false
}

// IsPermanent reports whether a failure is explicitly classified as not retryable.
func IsPermanent(err error) bool {
	return stderr.Is(err, ErrPermanent)
}

type classified struct {
	kind error
	err  error
}

func (e *classified) Error() string { return e.kind.Error() + ": " + e.err.Error() }

func (e *classified) Unwrap() error { return e.err }

func (e *classified) Is(target error) bool { return target == e.kind }
