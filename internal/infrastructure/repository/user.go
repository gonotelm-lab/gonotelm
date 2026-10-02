package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserRepositoryImpl struct {
	userStore database.UserStore
}

var _ identityrepo.UserRepository = &UserRepositoryImpl{}

func NewUserRepository(userStore database.UserStore) identityrepo.UserRepository {
	return &UserRepositoryImpl{userStore: userStore}
}

func (r *UserRepositoryImpl) Save(ctx context.Context, user *identityentity.User) error {
	if err := r.userStore.Upsert(ctx, mapper.UserToSchema(user)); err != nil {
		return errors.WithMessage(err, "failed to save user")
	}

	return nil
}

func (r *UserRepositoryImpl) GetById(ctx context.Context, id valobj.Uid) (*identityentity.User, error) {
	sch, err := r.userStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identityerrors.ErrUserNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user by id")
	}

	return mapper.UserFromSchema(sch), nil
}

func (r *UserRepositoryImpl) GetByProviderSub(
	ctx context.Context,
	issuer identityentity.ProviderType,
	sub string,
) (*identityentity.User, error) {
	sch, err := r.userStore.GetByProviderAndSub(ctx, string(issuer), sub)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identityerrors.ErrUserNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user by issuer and sub")
	}

	return mapper.UserFromSchema(sch), nil
}
