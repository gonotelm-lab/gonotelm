// Package resend implements mail.Sender on top of the Resend HTTP API.
//
//	API: https://resend.com/docs/api-reference/emails/send-email
//	SDK: https://github.com/resend/resend-go
package resend

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	stderr "errors"
	"fmt"
	"net"
	"net/http"
	netmail "net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
	"github.com/resend/resend-go/v4"
)

const (
	// defaultTimeout applies only when no http client is supplied; normal wiring
	// passes the project-wide pkg/httpclient (tracing + transport-level retries).
	defaultTimeout = 10 * time.Second

	// quoteTriggers are the characters that force quoting in a display name.
	quoteTriggers = `()<>[]:;@\,."`
)

type Sender struct {
	client *resend.Client
	config mail.Config
}

var _ mail.Sender = (*Sender)(nil)

// New builds the Resend sender. A missing api key or From is an error: wiring
// mistakes must surface at startup, not when a user clicks "register".
func New(c mail.Config, apiKey string, opts ...Option) (*Sender, error) {
	from, err := bareAddress(c.From)
	if err != nil {
		return nil, err
	}
	c.From = from

	if strings.TrimSpace(apiKey) == "" {
		return nil, stderr.New("resend: api key is required")
	}

	o := buildOptions(opts...)

	httpClient := o.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	client := resend.NewCustomClient(httpClient, strings.TrimSpace(apiKey))
	if o.baseURL != "" {
		baseURL, err := url.Parse(o.baseURL)
		if err != nil {
			return nil, fmt.Errorf("resend: invalid base url %q: %w", o.baseURL, err)
		}
		client.BaseURL = baseURL
	}

	return &Sender{client: client, config: c}, nil
}

// bareAddress rejects anything that is not a bare email address. A display name
// belongs in Config.FromName — accepting "Name <addr>" here would send
// "Name <Name <addr>>", and a bare domain is not an address at all.
func bareAddress(raw string) (string, error) {
	from := strings.TrimSpace(raw)
	if from == "" {
		return "", stderr.New("resend: from address is required")
	}

	parsed, err := netmail.ParseAddress(from)
	if err != nil {
		return "", fmt.Errorf("resend: invalid from address %q, want no-reply@your-domain.com: %w", from, err)
	}
	if parsed.Address != from {
		return "", fmt.Errorf("resend: from must be a bare address, got %q, put the display name in fromName", from)
	}

	return from, nil
}

func (s *Sender) Send(ctx context.Context, msg mail.Message) (*mail.Result, error) {
	if len(msg.To) == 0 {
		return nil, mail.Permanent(stderr.New("resend: no recipient"))
	}
	if msg.Subject == "" {
		return nil, mail.Permanent(stderr.New("resend: empty subject"))
	}

	params := &resend.SendEmailRequest{
		From:    s.from(),
		To:      msg.To,
		Subject: msg.Subject,
		Html:    msg.HTML,
		Text:    msg.Text,
		Headers: msg.Headers,
	}

	resp, err := s.client.Emails.SendWithOptions(ctx, params, &resend.SendEmailOptions{
		IdempotencyKey: idempotencyKey(msg),
	})
	if err != nil {
		return nil, classify(err)
	}

	return &mail.Result{Id: resp.Id}, nil
}

// idempotencyKey keeps transport-level retries of one Send from delivering twice.
// A deterministic caller-supplied value also covers business-level resends.
func idempotencyKey(msg mail.Message) string {
	if msg.IdempotencyKey != "" {
		return msg.IdempotencyKey
	}

	buf := make([]byte, 16)
	if _, err := crand.Read(buf); err != nil {
		return ""
	}

	return hex.EncodeToString(buf)
}

func (s *Sender) from() string {
	if s.config.FromName == "" {
		return s.config.From
	}

	return formatAddress(s.config.FromName, s.config.From)
}

// formatAddress builds the RFC 5322 "display name <addr>" form, quoting and
// escaping the display name when it contains specials.
func formatAddress(name, addr string) string {
	if name == "" {
		return addr
	}

	if strings.ContainsAny(name, quoteTriggers) ||
		strings.HasPrefix(name, " ") ||
		strings.HasSuffix(name, " ") {
		name = `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name) + `"`
	}

	return name + " <" + addr + ">"
}

// classify maps SDK errors onto retryability. pkg/httpclient has already retried
// 5xx / 429 / 408 at the transport level, so anything reaching here is out of
// retries: default to permanent and only keep explicit network errors transient.
func classify(err error) error {
	var rateLimit *resend.RateLimitError
	if stderr.As(err, &rateLimit) {
		return &mail.RateLimitError{
			Message:    rateLimit.Message,
			RetryAfter: parseRetryAfter(rateLimit.RetryAfter),
		}
	}

	if stderr.Is(err, context.Canceled) {
		return err
	}
	if stderr.Is(err, context.DeadlineExceeded) {
		return mail.Transient(err)
	}

	var netErr net.Error
	if stderr.As(err, &netErr) {
		return mail.Transient(err)
	}

	return mail.Permanent(err)
}

// parseRetryAfter reads the seconds form of Retry-After (RFC 9110 also allows an
// HTTP date, which we ignore).
func parseRetryAfter(raw string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
