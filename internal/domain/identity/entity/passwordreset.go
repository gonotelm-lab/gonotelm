package entity

import (
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	// PasswordResetStateTTL 找回密码链接有效期
	// 链接一被用掉就能改密码，所以只给 30 分钟
	PasswordResetStateTTL        = 30 * time.Minute
	PasswordResetLinkTokenLength = 64
	// PasswordResetSendCooldown 同一邮箱两次发送邮件的最小间隔，跟注册一致
	PasswordResetSendCooldown = SignupSendCooldown
)

// PasswordResetState 待确认的找回密码请求
// 跟注册 state 不同：不带密码，只有邮箱与一条一次性链接 token
type PasswordResetState struct {
	Email         string
	LinkTokenHash string
	CreatedAt     valobj.Time
}

func NewPasswordResetState(email string) *PasswordResetState {
	return &PasswordResetState{
		Email:     NormalizeEmail(email),
		CreatedAt: valobj.NewTime(),
	}
}

// IssueCredentials 生成链接 token，只把哈希写回记录，明文只出现在邮件里
func (p *PasswordResetState) IssueCredentials() (linkToken string, err error) {
	linkToken, err = idp.RandomString(PasswordResetLinkTokenLength)
	if err != nil {
		return "", err
	}

	p.LinkTokenHash = HashSecret(linkToken)

	return linkToken, nil
}
