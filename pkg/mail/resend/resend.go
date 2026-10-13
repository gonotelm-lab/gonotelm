// Package resend implements mail.Sender with the Resend HTTP API
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
	// defaultTimeout is used only when no http client is passed in
	// normal wiring passes pkg/httpclient
	defaultTimeout = 10 * time.Second

	// quoteTriggers force quoting in a display name
	quoteTriggers = `()<>[]:;@\,."`
)

type Sender struct {
	client *resend.Client
	config mail.Config
}

var _ mail.Sender = (*Sender)(nil)

// New creates the Resend sender, a missing api key or From is a startup error
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

// bareAddress accepts only a bare email address, the display name goes in Config.FromName
// passing "Name <addr>" here would send "Name <Name <addr>>", a bare domain is not an address
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

// idempotencyKey dedupes transport-level retries of one Send
// a caller-supplied value also covers business-level resends
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

// formatAddress creates the RFC 5322 "display name <addr>" form, quoting and escaping specials in the name
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

// classify marks SDK errors transient or permanent, pkg/httpclient already retried 5xx / 429 / 408
// errors reaching here are past retries, only deadline and network errors stay transient
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

// parseRetryAfter reads the seconds form of Retry-After, the HTTP date form in RFC 9110 is ignored
func parseRetryAfter(raw string) time.Duration {
	seconds, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || seconds <= 0 {
		return 0
	}

	return time.Duration(seconds) * time.Second
}
