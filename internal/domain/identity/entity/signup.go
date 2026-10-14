package entity

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/idp"
)

const (
	SignupStateTTL        = 24 * time.Hour
	SignupCodeTTL         = 15 * time.Minute
	SignupMaxCodeAttempts = 5
	// SignupSendCooldown 同一邮箱两次发验证邮件的最小间隔
	SignupSendCooldown = 60 * time.Second
	// SignupTicketLength 注册票据的随机字节长度
	SignupTicketLength = 32

	MinPasswordBytes = 8
	// bcrypt 只使用前 72 字节，超长直接拒绝
	MaxPasswordBytes = 72
)

// SignupState 待验证的邮箱注册，验证通过后才创建账号
type SignupState struct {
	Email        string
	PasswordHash string
	TicketHash   string
	CodeHash     string
	CodeExpireAt valobj.Time
	CreatedAt    valobj.Time
}

// NewSignupTicket 生成注册票据明文，记录里只保存 HashSecret(ticket)
func NewSignupTicket() (string, error) {
	return idp.RandomString(SignupTicketLength)
}

func NewSignupState(email, passwordHash, ticket string) *SignupState {
	return &SignupState{
		Email:        NormalizeEmail(email),
		PasswordHash: passwordHash,
		TicketHash:   HashSecret(ticket),
		CreatedAt:    valobj.NewTime(),
	}
}

// IssueCredentials 生成 6 位验证码，只把哈希写回记录，明文只出现在邮件里
func (p *SignupState) IssueCredentials() (code string, err error) {
	code, err = randomVerificationCode()
	if err != nil {
		return "", err
	}

	p.CodeHash = HashSecret(code)
	p.CodeExpireAt = valobj.NewTimeFrom(time.Now().Add(SignupCodeTTL).UnixMilli())

	return code, nil
}

// MatchesTicket 注册 ticket 只属于发起注册的那个浏览器，匹配才写入密码
func (p *SignupState) MatchesTicket(ticket string) bool {
	return p.TicketHash != "" && constantTimeEqual(p.TicketHash, HashSecret(ticket))
}

// RotateCredentials 轮换注册票据，并把记录里的密码哈希换成本次请求提供的，返回新票据明文
// 票据与密码必须同源：只换票据，完成验证的一方就会把别人先前写进记录的密码写进账号
func (p *SignupState) RotateCredentials(passwordHash string) (string, error) {
	ticket, err := NewSignupTicket()
	if err != nil {
		return "", err
	}

	p.TicketHash = HashSecret(ticket)
	p.PasswordHash = passwordHash

	return ticket, nil
}

func (p *SignupState) MatchesCode(code string) bool {
	if p.IsCodeExpired(time.Now()) {
		return false
	}

	return constantTimeEqual(p.CodeHash, HashSecret(code))
}

func (p *SignupState) IsCodeExpired(now time.Time) bool {
	return now.After(p.CodeExpireAt.Time())
}

func HashSecret(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// 邮箱形状：local part 按 HTML5 规范去掉反引号，域名带点且 TLD 为字母
// 不接受 IP 字面量与 Unicode（IDN）地址，统一按 ASCII 小写处理
var emailRegexp = regexp.MustCompile(
	`^[A-Za-z0-9.!#$%&'*+/=?^_{|}~-]+@(?:[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?\.)+[A-Za-z]{2,63}$`,
)

// ValidateEmail 只做格式粗筛，邮箱真实性由验证链接/验证码证明
func ValidateEmail(email string) error {
	if email == "" || len(email) > 255 || !emailRegexp.MatchString(email) {
		return identerr.ErrEmailInvalid
	}

	return nil
}

// ValidatePassword 长度下限防弱密码，上限对齐 bcrypt
func ValidatePassword(plain string) error {
	if len(plain) < MinPasswordBytes || len(plain) > MaxPasswordBytes {
		return identerr.ErrWeakPassword
	}

	return nil
}

// ValidateComplexPassword 复杂密码校验，要求含大写、小写和数字
// 是否启用由部署配置决定
func ValidateComplexPassword(plain string) error {
	if err := ValidatePassword(plain); err != nil {
		return err
	}

	if !hasUpperLowerDigit(plain) {
		return identerr.ErrWeakPassword
	}

	return nil
}

var (
	upperLetterRegexp = regexp.MustCompile(`[A-Z]`)
	lowerLetterRegexp = regexp.MustCompile(`[a-z]`)
	digitRegexp       = regexp.MustCompile(`[0-9]`)
)

func hasUpperLowerDigit(plain string) bool {
	return upperLetterRegexp.MatchString(plain) &&
		lowerLetterRegexp.MatchString(plain) &&
		digitRegexp.MatchString(plain)
}

// randomVerificationCode 6 位数字验证码，注册与找回密码共用
func randomVerificationCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}
