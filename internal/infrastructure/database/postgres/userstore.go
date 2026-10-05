package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type UserStoreImpl struct {
	db *gorm.DB
}

var _ database.UserStore = &UserStoreImpl{}

func NewUserStoreImpl(db *gorm.DB) *UserStoreImpl {
	return &UserStoreImpl{db: db}
}

func (s *UserStoreImpl) Create(ctx context.Context, user *schema.User) error {
	if err := s.db.WithContext(ctx).Create(user).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserStoreImpl) Upsert(ctx context.Context, user *schema.User) error {
	cl := clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"email":      user.Email,
			"nickname":   user.Nickname,
			"status":     user.Status,
			"avatar":     user.Avatar,
			"updated_at": user.UpdatedAt,
		}),
	}
	if err := s.db.WithContext(ctx).
		Model(&schema.User{}).
		Clauses(cl).
		Create(user).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *UserStoreImpl) GetById(ctx context.Context, id valobj.Uid) (*schema.User, error) {
	var user schema.User
	err := s.db.WithContext(ctx).
		Where("id = ?", id).
		Take(&user).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &user, nil
}

func (s *UserStoreImpl) GetByProviderAndSub(
	ctx context.Context,
	provider, sub string,
) (*schema.User, error) {
	var user schema.User
	err := s.db.WithContext(ctx).
		Where("provider = ? AND sub = ?", provider, sub).
		Take(&user).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &user, nil
}
