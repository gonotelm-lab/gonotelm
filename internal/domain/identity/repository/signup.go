package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
)

// SignupRepository 邮箱注册待验证记录的存取；记录本身是短时状态，落在 Redis。
type SignupRepository interface {
	// Save 写入记录与链接索引；previousLinkTokenHash 非空时旧链接立即失效。
	Save(ctx context.Context, pending *entity.SignupPending, previousLinkTokenHash string) error
	GetByEmail(ctx context.Context, email string) (*entity.SignupPending, error)
	GetByLinkToken(ctx context.Context, linkToken string) (*entity.SignupPending, error)
	DeleteByEmail(ctx context.Context, email string) error
	IncrAttempts(ctx context.Context, email string) (int64, error)
	// StartSendCooldown 启动发信冷却窗口；冷却期内返回 ErrTooManyRequests。
	StartSendCooldown(ctx context.Context, email string) error
	// ClearSendCooldown 清除发信冷却，用于发送失败后允许立即重试。
	ClearSendCooldown(ctx context.Context, email string) error
}
