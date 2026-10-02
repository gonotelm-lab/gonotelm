package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type UserAvatar string

type UserStatus string

const (
	UserStatusActive UserStatus = "active"
	UserStatusBanned UserStatus = "banned"
)

type User struct {
	Id        valobj.Uid
	Avatar    UserAvatar
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

func (u *User) SetEmail(email string) {
	u.Email = email
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) SetNickname(nickname string) {
	u.Nickname = nickname
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) SetAvatar(avatar UserAvatar) {
	u.Avatar = avatar
	u.UpdatedAt = valobj.NewTime()
}

func (u *User) Activate() {
	u.Status = UserStatusActive
}

func (u *User) Ban() {
	u.Status = UserStatusBanned
}
