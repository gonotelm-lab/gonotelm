package adapter

import (
	"context"

	coreadapter "github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	identrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// IdentityMailer 实现身份域的两类事务邮件端口：注册发验证码，找回密码发链接
// 选模板、定主题、派生幂等键是业务形态，真正的发送交给通用的 core/adapter.Mailer
type IdentityMailer struct {
	mailer coreadapter.Mailer
}

var (
	_ identrepo.EmailVerificationMailer = (*IdentityMailer)(nil)
	_ identrepo.PasswordResetMailer     = (*IdentityMailer)(nil)
)

func NewIdentityMailer(mailer coreadapter.Mailer) *IdentityMailer {
	return &IdentityMailer{mailer: mailer}
}

func (m *IdentityMailer) SendEmailVerification(ctx context.Context, to, code string) error {
	msg, err := renderEmailVerification(to, code)
	if err != nil {
		return errors.WithMessage(err, "render email verification failed")
	}

	if err := m.mailer.Send(ctx, msg); err != nil {
		return errors.WithMessage(err, "send email verification failed")
	}

	return nil
}

func (m *IdentityMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	msg, err := renderPasswordReset(to, link)
	if err != nil {
		return errors.WithMessage(err, "render password reset email failed")
	}

	if err := m.mailer.Send(ctx, msg); err != nil {
		return errors.WithMessage(err, "send password reset email failed")
	}

	return nil
}
