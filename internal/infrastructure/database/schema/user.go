package schema

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type User struct {
	Id              valobj.Uid `gorm:"column:id"`
	Email           string     `gorm:"column:email"`
	EmailVerifiedAt int64      `gorm:"column:email_verified_at"`
	Nickname        string     `gorm:"column:nickname"`
	Status          string     `gorm:"column:status"`
	Avatar          string     `gorm:"column:avatar"`
	PasswordHash    string     `gorm:"column:password_hash"`
	CreatedAt       int64      `gorm:"column:created_at"`
	UpdatedAt       int64      `gorm:"column:updated_at"`
}

func (User) TableName() string {
	return "users_v2"
}

type UserUpdateProfileParams struct {
	Id        valobj.Uid
	Nickname  string
	UpdatedAt int64
}

type UserUpdateAvatarParams struct {
	Id        valobj.Uid
	Avatar    string
	UpdatedAt int64
}

type UserUpdatePasswordParams struct {
	Id           valobj.Uid
	PasswordHash string
	UpdatedAt    int64
}
