package adapter

import "context"

// Mailer 事务性邮件发送端口
type Mailer interface {
	// SendEmailVerification 发送邮箱验证邮件，邮件正文同时包含验证链接与 6 位验证码
	SendEmailVerification(ctx context.Context, to, link, code string) error
}
