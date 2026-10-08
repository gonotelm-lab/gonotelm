package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	domainerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func UserToSchema(u *domainerr.User) *schema.User {
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

func UserFromSchema(u *schema.User) (*domainerr.User, error) {
	avatar, err := valobj.DecodeStoreKey(u.Avatar)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user avatar failed, user_id=%s", u.Id)
	}

	return &domainerr.User{
		Id:        u.Id,
		Avatar:    avatar,
		Status:    domainerr.UserStatus(u.Status),
		Nickname:  u.Nickname,
		Email:     u.Email,
		Provider:  domainerr.ProviderType(u.Provider),
		Sub:       u.Sub,
		CreatedAt: valobj.NewTimeFrom(u.CreatedAt),
		UpdatedAt: valobj.NewTimeFrom(u.UpdatedAt),
	}, nil
}

func UserToCacheSchema(u *domainerr.User) *cacheschema.User {
	return &cacheschema.User{
		Id:        u.Id.String(),
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

func UserFromCacheSchema(u *cacheschema.User) (*domainerr.User, error) {
	id, err := valobj.NewUidFromString(u.Id)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user id failed, user_id=%s", u.Id)
	}

	avatar, err := valobj.DecodeStoreKey(u.Avatar)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user avatar failed, user_id=%s", u.Id)
	}

	return &domainerr.User{
		Id:        id,
		Avatar:    avatar,
		Status:    domainerr.UserStatus(u.Status),
		Nickname:  u.Nickname,
		Email:     u.Email,
		Provider:  domainerr.ProviderType(u.Provider),
		Sub:       u.Sub,
		CreatedAt: valobj.NewTimeFrom(u.CreatedAt),
		UpdatedAt: valobj.NewTimeFrom(u.UpdatedAt),
	}, nil
}
