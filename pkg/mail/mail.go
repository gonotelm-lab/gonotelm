// Package mail is the transport-agnostic abstraction for transactional email.
//
// Callers describe what to send (Message); a Sender decides how (Resend HTTP API,
// log-only dev sender, ...). Like pkg/idp, this package never imports internal/;
// provider implementations live in subpackages such as pkg/mail/resend.
package mail

import (
	"context"
	"strings"
)

// Type names a mail sender implementation. Implementations live in subpackages,
// next to the provider they talk to.
type Type string

const (
	// TypeLog writes the email to the log instead of sending it. It is also the
	// fallback whenever another type cannot be built.
	TypeLog Type = "log"
	// TypeResend sends through the Resend HTTP API.
	TypeResend Type = "resend"
)

// ParseType normalizes a configured type; an empty value means TypeLog.
func ParseType(raw string) Type {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return TypeLog
	}

	return Type(raw)
}

// Config is the sending identity of one Sender. Two senders (no-reply@ and
// support@) mean two Senders.
type Config struct {
	From     string // must be on a domain verified by the provider
	FromName string // display name; empty means From only
}

// Message is a rendered email.
type Message struct {
	To      []string
	Subject string
	HTML    string // HTML or Text must be set
	Text    string
	Headers map[string]string // extra headers, e.g. Auto-Submitted: auto-generated

	// IdempotencyKey deduplicates the same email upstream: reusing one value across
	// transport or business retries prevents double delivery. Empty means the
	// implementation generates a random key, which only covers retries within one
	// Send call.
	IdempotencyKey string
}

// Result is the upstream delivery id, kept for troubleshooting a delivery.
type Result struct {
	Id string
}

// Sender hands one email to an upstream. Use IsTransient to decide whether a
// failure is worth retrying.
type Sender interface {
	Send(ctx context.Context, msg Message) (*Result, error)
}
