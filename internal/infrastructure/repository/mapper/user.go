package mapper

import (
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

func UserToSchema(u *identityentity.User) *schema.User {
	return UserToSchemaWithPassword(u, "")
}

func UserToSchemaWithPassword(u *identityentity.User, passwordHash string) *schema.User {
	return &schema.User{
		Id:              u.Id,
		Email:           u.Email,
		EmailVerifiedAt: u.EmailVerifiedAt.Value(),
		Nickname:        u.Nickname,
		Status:          string(u.Status),
		Avatar:          u.Avatar.Encode(),
		PasswordHash:    passwordHash,
		CreatedAt:       u.CreatedAt.Value(),
		UpdatedAt:       u.UpdatedAt.Value(),
	}
}

// UserFromSchema 有意丢弃 password_hash：密码只经 UserRepository 的密码方法读写。
func UserFromSchema(u *schema.User) (*identityentity.User, error) {
	avatar, err := valobj.DecodeStoreKey(u.Avatar)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user avatar failed, user_id=%s", u.Id)
	}

	return &identityentity.User{
		Id:              u.Id,
		Email:           u.Email,
		EmailVerifiedAt: valobj.NewTimeFrom(u.EmailVerifiedAt),
		Nickname:        u.Nickname,
		Status:          identityentity.UserStatus(u.Status),
		Avatar:          avatar,
		CreatedAt:       valobj.NewTimeFrom(u.CreatedAt),
		UpdatedAt:       valobj.NewTimeFrom(u.UpdatedAt),
	}, nil
}

func UserToCacheSchema(u *identityentity.User) *cacheschema.User {
	return &cacheschema.User{
		Id:              u.Id.String(),
		Email:           u.Email,
		EmailVerifiedAt: u.EmailVerifiedAt.Value(),
		Nickname:        u.Nickname,
		Status:          string(u.Status),
		Avatar:          u.Avatar.Encode(),
		CreatedAt:       u.CreatedAt.Value(),
		UpdatedAt:       u.UpdatedAt.Value(),
	}
}

func UserFromCacheSchema(u *cacheschema.User) (*identityentity.User, error) {
	id, err := valobj.NewUidFromString(u.Id)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user id failed, user_id=%s", u.Id)
	}

	avatar, err := valobj.DecodeStoreKey(u.Avatar)
	if err != nil {
		return nil, errors.WithMessagef(err, "decode user avatar failed, user_id=%s", u.Id)
	}

	return &identityentity.User{
		Id:              id,
		Email:           u.Email,
		EmailVerifiedAt: valobj.NewTimeFrom(u.EmailVerifiedAt),
		Nickname:        u.Nickname,
		Status:          identityentity.UserStatus(u.Status),
		Avatar:          avatar,
		CreatedAt:       valobj.NewTimeFrom(u.CreatedAt),
		UpdatedAt:       valobj.NewTimeFrom(u.UpdatedAt),
	}, nil
}
