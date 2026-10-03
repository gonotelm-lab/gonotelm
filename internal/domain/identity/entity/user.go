package entity

import (
	"strings"
	"unicode/utf8"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
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

type User struct {
	Id        valobj.Uid
	Avatar    valobj.StoreKey
	Status    UserStatus
	Nickname  string
	Email     string
	Provider  ProviderType
	Sub       string
	CreatedAt valobj.Time
	UpdatedAt valobj.Time
}

func (u *User) normalizeNickname() {
	nickname := pkgstr.TruncateRune(u.Nickname, MaxUserNickNameRune)
	if nickname == "" {
		// assign a default name
		nickname = defaultUserNicknamePrefix + pkgstr.LastRune(u.Id.String(), 6)
	}

	u.Nickname = nickname
}

func NewUser(nickname string, provider ProviderType, subject string) *User {
	now := valobj.NewTime()
	u := &User{
		Id:        valobj.NewUid(),
		Nickname:  nickname,
		Provider:  provider,
		Sub:       subject,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    UserStatusActive,
	}

	u.normalizeNickname()

	return u
}

// NewAvatarKey 产出新的头像 StoreKey（isPublic=true，即公有读桶）
func (u *User) NewAvatarKey(keyFactory adapter.StoreKeyFactory) (valobj.StoreKey, error) {
	return keyFactory.New(userAvatarObjectPathPrefix+valobj.NewUnOrderedId().String(), true)
}

func (u *User) SetAvatar(avatar valobj.StoreKey) {
	u.Avatar = avatar
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) SetEmail(email string) {
	u.Email = email
	u.UpdatedAt = valobj.NewTime()
}

// SetNickname 空白或超长的昵称视为非法,不做静默截断/兜底。
func (u *User) SetNickname(nickname string) error {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" || utf8.RuneCountInString(nickname) > MaxUserNickNameRune {
		return identityerrors.ErrInvalidNickname
	}

	u.Nickname = nickname
	u.UpdatedAt = valobj.NewTime()

	return nil
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
