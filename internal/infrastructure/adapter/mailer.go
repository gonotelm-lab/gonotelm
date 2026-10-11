package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

// MailerImpl implements core/adapter.Mailer: it renders the business semantics
// (verification link + code) into an email and hands it to a mail.Sender. The body
// lives here only once, so swapping the transport never changes the email.
type MailerImpl struct {
	sender mail.Sender
}

var _ adapter.Mailer = (*MailerImpl)(nil)

func NewMailer(sender mail.Sender) *MailerImpl {
	return &MailerImpl{sender: sender}
}

func (m *MailerImpl) SendEmailVerification(ctx context.Context, to, link, code string) error {
	msg, err := renderEmailVerification(to, link, code)
	if err != nil {
		return errors.WithMessage(err, "render email verification failed")
	}

	if _, err := m.sender.Send(ctx, msg); err != nil {
		return errors.WithMessage(err, "send email verification failed")
	}

	return nil
}
