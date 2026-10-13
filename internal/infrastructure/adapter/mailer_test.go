package adapter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

type recordingSender struct {
	sent []mail.Message
	err  error
}

func (r *recordingSender) Send(_ context.Context, msg mail.Message) (*mail.Result, error) {
	r.sent = append(r.sent, msg)
	if r.err != nil {
		return nil, r.err
	}

	return &mail.Result{Id: "rec-1"}, nil
}

func TestMailerSendEmailVerificationRendersBothBodies(t *testing.T) {
	rec := &recordingSender{}
	link := "https://app.example.com/verify-email#token=abc123"

	err := NewMailer(rec).SendEmailVerification(t.Context(), "user@example.com", link, "123456")
	if err != nil {
		t.Fatal(err)
	}

	if len(rec.sent) != 1 {
		t.Fatalf("want 1 sent mail, got %d", len(rec.sent))
	}

	msg := rec.sent[0]
	if len(msg.To) != 1 || msg.To[0] != "user@example.com" {
		t.Fatalf("unexpected recipients: %v", msg.To)
	}
	if msg.Subject != verifyEmailSubject {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	if msg.Headers["Auto-Submitted"] != "auto-generated" {
		t.Fatalf("unexpected headers: %v", msg.Headers)
	}
	if msg.IdempotencyKey == "" {
		t.Fatal("idempotency key must not be empty")
	}

	for name, body := range map[string]string{"html": msg.HTML, "text": msg.Text} {
		if !strings.Contains(body, link) {
			t.Fatalf("%s body lost the link: %s", name, body)
		}
		if !strings.Contains(body, "123456") {
			t.Fatalf("%s body lost the code", name)
		}
	}
}

func TestMailerKeepsIdempotencyKeyPerCredentials(t *testing.T) {
	rec := &recordingSender{}
	mailer := NewMailer(rec)

	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", "https://a.example.com/#token=1", "111111")
	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", "https://a.example.com/#token=1", "111111")
	// 重发轮换凭证必须拿到新 key，否则第二封会被上游当成重复邮件丢掉
	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", "https://a.example.com/#token=2", "222222")

	if rec.sent[0].IdempotencyKey != rec.sent[1].IdempotencyKey {
		t.Fatal("same credentials must reuse the same idempotency key")
	}
	if rec.sent[0].IdempotencyKey == rec.sent[2].IdempotencyKey {
		t.Fatal("rotated credentials must produce a new idempotency key")
	}
}

func TestMailerPropagatesSenderError(t *testing.T) {
	sentinel := errors.New("boom")

	err := NewMailer(&recordingSender{err: sentinel}).SendEmailVerification(
		t.Context(), "user@example.com", "https://a.example.com/#token=1", "111111",
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want the sender error, got %v", err)
	}
}
