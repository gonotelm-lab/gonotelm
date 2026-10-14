package adapter

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

// recordingMailer 充当通用的 core/adapter.Mailer，只记录渲染结果
type recordingMailer struct {
	sent []mail.Message
	err  error
}

func (r *recordingMailer) Send(_ context.Context, msg mail.Message) error {
	r.sent = append(r.sent, msg)

	return r.err
}

func TestIdentityMailerSendEmailVerificationRendersBothBodies(t *testing.T) {
	rec := &recordingMailer{}

	err := NewIdentityMailer(rec).SendEmailVerification(t.Context(), "user@example.com", "123456")
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
		if !strings.Contains(body, "123456") {
			t.Fatalf("%s body lost the code", name)
		}
		// 邮件里只剩验证码，不该再出现任何链接
		if strings.Contains(body, "href=\"http") {
			t.Fatalf("%s body still carries a link", name)
		}
	}
}

func TestIdentityMailerKeepsIdempotencyKeyPerCredentials(t *testing.T) {
	rec := &recordingMailer{}
	mailer := NewIdentityMailer(rec)

	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", "111111")
	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", "111111")
	// 重发轮换验证码必须拿到新 key，否则第二封会被上游当成重复邮件丢掉
	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", "222222")

	if rec.sent[0].IdempotencyKey != rec.sent[1].IdempotencyKey {
		t.Fatal("same credentials must reuse the same idempotency key")
	}
	if rec.sent[0].IdempotencyKey == rec.sent[2].IdempotencyKey {
		t.Fatal("rotated credentials must produce a new idempotency key")
	}
}

func TestIdentityMailerPropagatesMailerError(t *testing.T) {
	sentinel := errors.New("boom")

	err := NewIdentityMailer(&recordingMailer{err: sentinel}).SendEmailVerification(
		t.Context(), "user@example.com", "111111",
	)
	if !errors.Is(err, sentinel) {
		t.Fatalf("want the mailer error, got %v", err)
	}
}

func TestIdentityMailerSendPasswordResetRendersBothBodies(t *testing.T) {
	rec := &recordingMailer{}
	link := "https://app.example.com/reset-password#token=abc123"

	err := NewIdentityMailer(rec).SendPasswordReset(t.Context(), "user@example.com", link)
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
	if msg.Subject != resetPasswordSubject {
		t.Fatalf("unexpected subject: %q", msg.Subject)
	}
	if msg.Subject == verifyEmailSubject {
		t.Fatal("the reset email must not reuse the verification subject")
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
	}
}

// 同一收件人、同一个验证码，两类邮件的幂等 key 也必须不同，
// 否则上游会把找回密码邮件当成验证邮件的重复投递直接丢掉
func TestIdentityMailerIdempotencyKeyDiffersByEmailKind(t *testing.T) {
	rec := &recordingMailer{}
	mailer := NewIdentityMailer(rec)
	const code = "111111"

	_ = mailer.SendEmailVerification(t.Context(), "user@example.com", code)
	_ = mailer.SendPasswordReset(t.Context(), "user@example.com", "https://a.example.com/#token=1")

	if len(rec.sent) != 2 {
		t.Fatalf("want 2 sent mails, got %d", len(rec.sent))
	}
	if rec.sent[0].IdempotencyKey == rec.sent[1].IdempotencyKey {
		t.Fatal("verify and reset emails must not share an idempotency key")
	}
}

func TestIdentityMailerPasswordResetIdempotencyKeyRotatesWithCredentials(t *testing.T) {
	rec := &recordingMailer{}
	mailer := NewIdentityMailer(rec)

	_ = mailer.SendPasswordReset(t.Context(), "user@example.com", "https://a.example.com/#token=1")
	_ = mailer.SendPasswordReset(t.Context(), "user@example.com", "https://a.example.com/#token=1")
	// 重发轮换 token 必须拿到新 key
	_ = mailer.SendPasswordReset(t.Context(), "user@example.com", "https://a.example.com/#token=2")

	if rec.sent[0].IdempotencyKey != rec.sent[1].IdempotencyKey {
		t.Fatal("same credentials must reuse the same idempotency key")
	}
	if rec.sent[0].IdempotencyKey == rec.sent[2].IdempotencyKey {
		t.Fatal("rotated credentials must produce a new idempotency key")
	}
}
