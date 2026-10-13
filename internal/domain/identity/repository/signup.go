package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// SignupRepository 邮箱注册待验证记录的读写，记录是短时状态，存在 Redis 里
type SignupRepository interface {
	// Save 写入记录，ttl 由实现按 SignupPendingTTL 固定
	Save(ctx context.Context, pending *entity.SignupPending) error
	GetByEmail(ctx context.Context, email string) (*entity.SignupPending, error)
	DeleteByEmail(ctx context.Context, email string) error
	IncrAttempts(ctx context.Context, email string) (int64, error)
	// StartSendCooldown 开始计时发邮件间隔，间隔内返回 ErrTooManyRequests
	StartSendCooldown(ctx context.Context, email string) error
	// ClearSendCooldown 清除发邮件间隔，发送失败后可立即重试
	ClearSendCooldown(ctx context.Context, email string) error
}
