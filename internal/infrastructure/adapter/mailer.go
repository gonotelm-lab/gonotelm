package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

// MailerImpl implements core/adapter.Mailer
// 注册发验证码（用户在验证页输入），找回密码发链接（点开就是重置页），两者形态不同
type MailerImpl struct {
	sender mail.Sender
}

var _ adapter.Mailer = (*MailerImpl)(nil)

func NewMailer(sender mail.Sender) *MailerImpl {
	return &MailerImpl{sender: sender}
}

func (m *MailerImpl) SendEmailVerification(ctx context.Context, to, code string) error {
	msg, err := renderEmailVerification(to, code)
	if err != nil {
		return errors.WithMessage(err, "render email verification failed")
	}

	if _, err := m.sender.Send(ctx, msg); err != nil {
		return errors.WithMessage(err, "send email verification failed")
	}

	return nil
}

func (m *MailerImpl) SendPasswordReset(ctx context.Context, to, link string) error {
	msg, err := renderPasswordReset(to, link)
	if err != nil {
		return errors.WithMessage(err, "render password reset email failed")
	}

	if _, err := m.sender.Send(ctx, msg); err != nil {
		return errors.WithMessage(err, "send password reset email failed")
	}

	return nil
}
