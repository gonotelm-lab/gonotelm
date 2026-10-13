package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// PasswordResetRepository 找回密码待确认记录的读写，记录是短时状态，存在 Redis 里
// 走链接而不是验证码，所以没有「验证码错误次数」这一项
type PasswordResetRepository interface {
	// Save 写入记录与链接索引，previousLinkTokenHash 非空时旧链接立即失效
	Save(ctx context.Context, pending *entity.PasswordResetPending, previousLinkTokenHash string) error
	GetByEmail(ctx context.Context, email string) (*entity.PasswordResetPending, error)
	GetByLinkToken(ctx context.Context, linkToken string) (*entity.PasswordResetPending, error)
	DeleteByEmail(ctx context.Context, email string) error
	// StartSendCooldown 开始计时发邮件间隔，间隔内返回 ErrTooManyRequests
	StartSendCooldown(ctx context.Context, email string) error
	// ClearSendCooldown 清除发邮件间隔，发送失败后可立即重试
	ClearSendCooldown(ctx context.Context, email string) error
}
