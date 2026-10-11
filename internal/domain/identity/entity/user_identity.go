package entity

import "github.com/gonotelm-lab/gonotelm/internal/core/valobj"

// UserIdentity 一个三方登录身份，(Provider, Subject) 全局唯一。
type UserIdentity struct {
	Id                    valobj.Id
	UserId                valobj.Uid
	Provider              ProviderType
	Subject               string
	ProviderEmail         string
	ProviderEmailVerified bool
	LastLoginAt           valobj.Time
	CreatedAt             valobj.Time
	UpdatedAt             valobj.Time
}

func NewUserIdentity(
	userId valobj.Uid,
	provider ProviderType,
	subject, providerEmail string,
	providerEmailVerified bool,
) *UserIdentity {
	now := valobj.NewTime()

	return &UserIdentity{
		Id:                    valobj.NewId(),
		UserId:                userId,
		Provider:              provider,
		Subject:               subject,
		ProviderEmail:         NormalizeEmail(providerEmail),
		ProviderEmailVerified: providerEmailVerified,
		LastLoginAt:           now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
}

func (i *UserIdentity) TouchLogin(at valobj.Time) {
	i.LastLoginAt = at
	i.UpdatedAt = at
}

func (i *UserIdentity) BelongsTo(userId valobj.Uid) bool {
	return i.UserId == userId
}
