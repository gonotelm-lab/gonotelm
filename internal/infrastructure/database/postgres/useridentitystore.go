package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type UserIdentityStoreImpl struct {
	db *gorm.DB
}

var _ database.UserIdentityStore = &UserIdentityStoreImpl{}

func NewUserIdentityStoreImpl(db *gorm.DB) *UserIdentityStoreImpl {
	return &UserIdentityStoreImpl{db: db}
}

func (s *UserIdentityStoreImpl) Create(ctx context.Context, identity *schema.UserIdentity) error {
	if err := dbFromContext(ctx, s.db).Create(identity).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserIdentityStoreImpl) GetByProviderAndSubject(
	ctx context.Context,
	provider, subject string,
) (*schema.UserIdentity, error) {
	var identity schema.UserIdentity
	err := dbFromContext(ctx, s.db).
		Where("provider = ? AND subject = ?", provider, subject).
		Take(&identity).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &identity, nil
}

func (s *UserIdentityStoreImpl) GetByUserIdAndId(
	ctx context.Context,
	userId valobj.Uid,
	id valobj.Id,
) (*schema.UserIdentity, error) {
	var identity schema.UserIdentity
	err := dbFromContext(ctx, s.db).
		Where("user_id = ? AND id = ?", userId, id).
		Take(&identity).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &identity, nil
}

func (s *UserIdentityStoreImpl) ListByUserId(
	ctx context.Context,
	userId valobj.Uid,
) ([]*schema.UserIdentity, error) {
	var identities []*schema.UserIdentity
	err := dbFromContext(ctx, s.db).
		Where("user_id = ?", userId).
		Order("created_at ASC").
		Find(&identities).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return identities, nil
}

func (s *UserIdentityStoreImpl) UpdateLastLoginAt(ctx context.Context, id valobj.Id, lastLoginAt int64) error {
	err := dbFromContext(ctx, s.db).
		Model(&schema.UserIdentity{}).
		Where("id = ?", id).
		Updates(map[string]any{"last_login_at": lastLoginAt, "updated_at": lastLoginAt}).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserIdentityStoreImpl) Delete(ctx context.Context, id valobj.Id) error {
	err := dbFromContext(ctx, s.db).
		Where("id = ?", id).
		Delete(&schema.UserIdentity{}).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserIdentityStoreImpl) DeleteByUserId(ctx context.Context, userId valobj.Uid) error {
	err := dbFromContext(ctx, s.db).
		Where("user_id = ?", userId).
		Delete(&schema.UserIdentity{}).Error
	if err != nil {
		return sql.WrapErr(err)
	}

	return nil
}
