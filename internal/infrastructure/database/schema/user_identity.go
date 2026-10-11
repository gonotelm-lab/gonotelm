package schema

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

type UserIdentity struct {
	Id                    valobj.Id  `gorm:"column:id"`
	UserId                valobj.Uid `gorm:"column:user_id"`
	Provider              string     `gorm:"column:provider"`
	Subject               string     `gorm:"column:subject"`
	ProviderEmail         string     `gorm:"column:provider_email"`
	ProviderEmailVerified bool       `gorm:"column:provider_email_verified"`
	LastLoginAt           int64      `gorm:"column:last_login_at"`
	CreatedAt             int64      `gorm:"column:created_at"`
	UpdatedAt             int64      `gorm:"column:updated_at"`
}

func (UserIdentity) TableName() string {
	return "user_identities_v2"
}
