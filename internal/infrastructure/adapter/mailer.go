package adapter

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/pkg/mail"
)

// MailerImpl 实现 core/adapter.Mailer：只做传输，不碰邮件内容
// 邮件形态（验证码 / 重置链接）由身份域的邮件端口决定，见 identitymailer.go
type MailerImpl struct {
	sender mail.Sender
}

var _ adapter.Mailer = (*MailerImpl)(nil)

func NewMailer(sender mail.Sender) *MailerImpl {
	return &MailerImpl{sender: sender}
}

func (m *MailerImpl) Send(ctx context.Context, msg mail.Message) error {
	_, err := m.sender.Send(ctx, msg)

	return err
}
