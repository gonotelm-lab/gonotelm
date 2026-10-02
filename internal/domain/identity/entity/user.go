package entity

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
)

type UserStatus string

const (
	UserStatusActive UserStatus = "active"
	UserStatusBanned UserStatus = "banned"
)

// 头像路径模板属于用户领域，桶名由 StoreKeyFactory 补（头像放公有桶）。
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

func NewUser(nickname string, provider ProviderType, subject string) *User {
	u := &User{
		Id:        valobj.NewUid(),
		Nickname:  nickname,
		Provider:  provider,
		Sub:       subject,
		CreatedAt: valobj.NewTime(),
		UpdatedAt: valobj.NewTime(),
		Status:    UserStatusActive,
	}

	return u
}

// NewAvatarKey 产出新的头像 StoreKey（isPublic=true，即公有读桶），不写入 u.Avatar。
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

func (u *User) SetNickname(nickname string) {
	u.Nickname = nickname
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) Activate() {
	u.Status = UserStatusActive
}

func (u *User) Ban() {
	u.Status = UserStatusBanned
}
