package mail

import (
	"context"
	stderr "errors"
	"fmt"
	"net"
	"time"
)

// Failure kinds, callers check IsTransient before retrying
var (
	// ErrTransient upstream temporarily unavailable: rate limit, network, timeout, 5xx
	ErrTransient = stderr.New("mail: transient failure")
	// ErrPermanent the email cannot be sent: bad address, unverified domain, rejected
	ErrPermanent = stderr.New("mail: permanent failure")
)

// RateLimitError upstream rate limited, RetryAfter is the suggested wait, 0 means upstream did not say
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

// Is makes RateLimitError match ErrTransient
func (e *RateLimitError) Is(target error) bool {
	return target == ErrTransient
}

// Transient marks err retryable and keeps the cause
func Transient(err error) error {
	if err == nil {
		return nil
	}

	return &classified{kind: ErrTransient, err: err}
}

// Permanent marks err not retryable and keeps the cause
func Permanent(err error) error {
	if err == nil {
		return nil
	}

	return &classified{kind: ErrPermanent, err: err}
}

// IsTransient checks if a failure is retryable
// context.Canceled is not, unclassified network errors are transient
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

// IsPermanent checks if a failure is permanent
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
