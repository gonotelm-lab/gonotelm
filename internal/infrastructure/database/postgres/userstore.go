package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type UserStoreImpl struct {
	db *gorm.DB
}

var _ database.UserStore = &UserStoreImpl{}

func NewUserStoreImpl(db *gorm.DB) *UserStoreImpl {
	return &UserStoreImpl{db: db}
}

func (s *UserStoreImpl) Create(ctx context.Context, user *schema.User) error {
	if err := dbFromContext(ctx, s.db).Create(user).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserStoreImpl) GetById(ctx context.Context, id valobj.Uid) (*schema.User, error) {
	var user schema.User
	err := dbFromContext(ctx, s.db).
		Where("id = ?", id).
		Take(&user).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &user, nil
}

func (s *UserStoreImpl) BatchGetById(ctx context.Context, ids []valobj.Uid) ([]*schema.User, error) {
	if len(ids) == 0 {
		return []*schema.User{}, nil
	}

	var users []*schema.User
	err := dbFromContext(ctx, s.db).
		Where("id IN ?", ids).
		Find(&users).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return users, nil
}

func (s *UserStoreImpl) GetByEmail(ctx context.Context, email string) (*schema.User, error) {
	var user schema.User
	err := dbFromContext(ctx, s.db).
		Where("email = ?", email).
		Take(&user).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &user, nil
}

func (s *UserStoreImpl) GetPasswordHash(ctx context.Context, id valobj.Uid) (string, error) {
	var row struct {
		PasswordHash string `gorm:"column:password_hash"`
	}
	err := dbFromContext(ctx, s.db).
		Model(&schema.User{}).
		Select("password_hash").
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		return "", sql.WrapErr(err)
	}

	return row.PasswordHash, nil
}

func (s *UserStoreImpl) UpdateProfile(ctx context.Context, params *schema.UserUpdateProfileParams) error {
	return s.update(ctx, params.Id, map[string]any{
		"nickname":   params.Nickname,
		"updated_at": params.UpdatedAt,
	})
}

func (s *UserStoreImpl) UpdateAvatar(ctx context.Context, params *schema.UserUpdateAvatarParams) error {
	return s.update(ctx, params.Id, map[string]any{
		"avatar":     params.Avatar,
		"updated_at": params.UpdatedAt,
	})
}

func (s *UserStoreImpl) UpdatePasswordHash(ctx context.Context, params *schema.UserUpdatePasswordParams) error {
	return s.update(ctx, params.Id, map[string]any{
		"password_hash": params.PasswordHash,
		"updated_at":    params.UpdatedAt,
	})
}

func (s *UserStoreImpl) update(ctx context.Context, id valobj.Uid, columns map[string]any) error {
	err := dbFromContext(ctx, s.db).
		Model(&schema.User{}).
		Where("id = ?", id).
		Updates(columns).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}
