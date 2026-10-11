package entity

import (
	"strings"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	pkgstr "github.com/gonotelm-lab/gonotelm/pkg/string"
)

const (
	MaxUserNickNameRune = 255

	defaultUserNicknamePrefix = "user_"
)

type UserStatus string

const (
	UserStatusActive UserStatus = "active"
	UserStatusBanned UserStatus = "banned"
)

const userAvatarObjectPathPrefix = "users/avatar/"

// User 账号主体
// 不包含密码
type User struct {
	Id              valobj.Uid
	Email           string
	EmailVerifiedAt valobj.Time
	Nickname        string
	Status          UserStatus
	Avatar          valobj.StoreKey
	CreatedAt       valobj.Time
	UpdatedAt       valobj.Time
}

// NormalizeEmail 全量小写 + 去空白；模型只做这一种归一化，不碰 . 与 + 别名。
func NormalizeEmail(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// NewUser 只在邮箱已被证明（provider verified 或验证链接/验证码通过）时调用。
func NewUser(email, nickname string, emailVerifiedAt valobj.Time) *User {
	now := valobj.NewTime()
	u := &User{
		Id:              valobj.NewUid(),
		Email:           NormalizeEmail(email),
		EmailVerifiedAt: emailVerifiedAt,
		Nickname:        nickname,
		Status:          UserStatusActive,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	u.normalizeNickname()

	return u
}

func (u *User) normalizeNickname() {
	nickname := pkgstr.TruncateRune(u.Nickname, MaxUserNickNameRune)
	if nickname == "" {
		// assign a default name
		nickname = defaultUserNicknamePrefix + pkgstr.LastRune(u.Id.String(), 6)
	}

	u.Nickname = nickname
}

// 生成一个新的头像存储Key
func (u *User) GenerateAvatarKey(keyFactory adapter.StoreKeyFactory) (valobj.StoreKey, error) {
	return keyFactory.New(userAvatarObjectPathPrefix+valobj.NewUnOrderedId().String(), true)
}

func (u *User) SetAvatar(avatar valobj.StoreKey) {
	u.Avatar = avatar
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) SetEmail(email string) {
	u.Email = NormalizeEmail(email)
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) MarkEmailVerified(at valobj.Time) {
	u.EmailVerifiedAt = at
	u.UpdatedAt = at
}

// SetNickname 空白或超长的昵称视为非法,不做静默截断/兜底。
func (u *User) SetNickname(nickname string) error {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > MaxUserNickNameRune {
		return identerr.ErrInvalidNickname
	}

	u.Nickname = nickname
	u.UpdatedAt = valobj.NewTime()

	return nil
}

func (u *User) IsEmailVerified() bool {
	return u.EmailVerifiedAt.Value() > 0
}

func (u *User) Activate() {
	u.Status = UserStatusActive
}

func (u *User) Ban() {
	u.Status = UserStatusBanned
}

func (u *User) IsActive() bool {
	return u.Status == UserStatusActive
}

func (u *User) IsBanned() bool {
	return u.Status == UserStatusBanned
}
