// Package mail sends transactional email, the Sender picks the transport
// callers pass a Message, implementations live in subpackages such as pkg/mail/resend
// pkg never imports internal
package mail

import (
	"context"
	"strings"
)

// Type is a mail sender name, implementations live in subpackages
type Type string

const (
	// TypeLog writes the email to the log, used when no real sender is built
	TypeLog Type = "log"
	// TypeResend sends through the Resend HTTP API
	TypeResend Type = "resend"
)

// ParseType trims and lowercases the configured type, empty means TypeLog
func ParseType(raw string) Type {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return TypeLog
	}

	return Type(raw)
}

// Config is the From address of a Sender, no-reply@ and support@ need two Senders
type Config struct {
	From     string // must be a domain verified by the provider
	FromName string // display name, empty means From only
}

// Message is a rendered email
type Message struct {
	To      []string
	Subject string
	HTML    string // HTML or Text must be set
	Text    string
	Headers map[string]string // extra headers, e.g. Auto-Submitted: auto-generated

	// IdempotencyKey dedupes one email upstream, one value across retries prevents double delivery
	// empty means a random key per Send call, covers retries inside that call only
	IdempotencyKey string
}

// Result is the upstream email id, for troubleshooting
type Result struct {
	Id string
}

// Sender sends one email, check the failure with IsTransient
type Sender interface {
	Send(ctx context.Context, msg Message) (*Result, error)
}
