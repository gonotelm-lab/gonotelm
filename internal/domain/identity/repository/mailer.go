package repository

import "context"

// EmailVerificationMailer 发送注册验证邮件：业务形态是 6 位验证码，用户在验证页输入
// 模板、主题与幂等键由实现决定，调用方只管给收件人与验证码
type EmailVerificationMailer interface {
	// SendEmailVerification 发送邮箱验证邮件，只带一个 6 位验证码
	SendEmailVerification(ctx context.Context, to, code string) error
}

// PasswordResetMailer 发送找回密码邮件：业务形态是重置链接，点开就是重置页
type PasswordResetMailer interface {
	// SendPasswordReset 发送找回密码邮件，只带重置链接
	SendPasswordReset(ctx context.Context, to, link string) error
}
