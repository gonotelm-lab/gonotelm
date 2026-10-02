package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func UserToSchema(u *identityentity.User) *schema.User {
	return &schema.User{
		Id:        u.Id,
		Email:     u.Email,
		Nickname:  u.Nickname,
		Status:    string(u.Status),
		Avatar:    u.Avatar.Encode(),
		Provider:  string(u.Provider),
		Sub:       u.Sub,
		CreatedAt: u.CreatedAt.Value(),
		UpdatedAt: u.UpdatedAt.Value(),
	}
}

func UserFromSchema(u *schema.User) (*identityentity.User, error) {
	avatar, err := valobj.DecodeStoreKey(u.Avatar)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user avatar failed, user_id=%s", u.Id)
	}

	return &identityentity.User{
		Id:        u.Id,
		Avatar:    avatar,
		Status:    identityentity.UserStatus(u.Status),
		Nickname:  u.Nickname,
		Email:     u.Email,
		Provider:  identityentity.ProviderType(u.Provider),
		Sub:       u.Sub,
		CreatedAt: valobj.NewTimeFrom(u.CreatedAt),
		UpdatedAt: valobj.NewTimeFrom(u.UpdatedAt),
	}, nil
}
