package adapter

import "context"

// Mailer 事务性邮件发送端口
// 两个流程的凭证形态不同：注册发验证码，找回密码发链接
type Mailer interface {
	// SendEmailVerification 发送邮箱验证邮件，只带一个 6 位验证码
	SendEmailVerification(ctx context.Context, to, code string) error
	// SendPasswordReset 发送找回密码邮件，只带重置链接
	SendPasswordReset(ctx context.Context, to, link string) error
}
