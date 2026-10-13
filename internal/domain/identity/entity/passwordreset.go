package entity

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	// PasswordResetPendingTTL 找回密码链接有效期
	// 链接一被用掉就能改密码，所以只给 30 分钟
	PasswordResetPendingTTL      = 30 * time.Minute
	PasswordResetLinkTokenLength = 64
	// PasswordResetSendCooldown 同一邮箱两次发信的最小间隔，跟注册一致
	PasswordResetSendCooldown = SignupSendCooldown
)

// PasswordResetPending 待确认的找回密码请求
// 跟注册 pending 不同：不带密码，只有邮箱与一条一次性链接 token
type PasswordResetPending struct {
	Email         string
	LinkTokenHash string
	CreatedAt     valobj.Time
}

func NewPasswordResetPending(email string) *PasswordResetPending {
	return &PasswordResetPending{
		Email:     NormalizeEmail(email),
		CreatedAt: valobj.NewTime(),
	}
}

// IssueCredentials 生成链接 token，只把哈希写回记录，明文只出现在邮件里
func (p *PasswordResetPending) IssueCredentials() (linkToken string, err error) {
	linkToken, err = idp.RandomString(PasswordResetLinkTokenLength)
	if err != nil {
		return "", err
	}

	p.LinkTokenHash = HashSecret(linkToken)

	return linkToken, nil
}
