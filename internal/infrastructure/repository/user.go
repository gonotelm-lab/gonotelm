package repository

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserRepositoryImpl struct {
	userStore database.UserStore
	cache     cache.UserCache
}

var _ identityrepo.UserRepository = &UserRepositoryImpl{}

func NewUserRepository(userStore database.UserStore, userCache cache.UserCache) identityrepo.UserRepository {
	return &UserRepositoryImpl{userStore: userStore, cache: userCache}
}

func (r *UserRepositoryImpl) Save(ctx context.Context, user *identityentity.User) error {
	if err := r.userStore.Upsert(ctx, mapper.UserToSchema(user)); err != nil {
		return errors.WithMessage(err, "failed to save user")
	}

	r.deleteCache(ctx, user.Id.String())

	return nil
}

func (r *UserRepositoryImpl) GetById(ctx context.Context, id valobj.Uid) (*identityentity.User, error) {
	cached, cacheErr := r.cacheGetById(ctx, id.String())
	if user, ok := r.decodeCache(ctx, cached, cacheErr); ok {
		return user, nil
	}

	sch, err := r.userStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identityerrors.ErrUserNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user by id")
	}

	user, err := mapper.UserFromSchema(sch)
	if err != nil {
		return nil, err
	}

	r.setCache(ctx, user)

	return user, nil
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

	return mapper.UserFromSchema(sch)
}

func (r *UserRepositoryImpl) cacheGetById(ctx context.Context, id string) (*cacheschema.User, error) {
	if r.cache == nil {
		return nil, nil
	}

	return r.cache.GetById(ctx, id)
}

func (r *UserRepositoryImpl) decodeCache(
	ctx context.Context, sch *cacheschema.User, cacheErr error,
) (*identityentity.User, bool) {
	if cacheErr != nil {
		slog.WarnContext(ctx, "get user from cache failed, fallback to store", slog.Any("err", cacheErr))

		return nil, false
	}
	if sch == nil {
		return nil, false
	}

	user, err := mapper.UserFromCacheSchema(sch)
	if err != nil {
		slog.WarnContext(ctx, "decode cached user failed, fallback to store",
			slog.String("user_id", sch.Id),
			slog.Any("err", err),
		)

		return nil, false
	}

	return user, true
}

func (r *UserRepositoryImpl) setCache(ctx context.Context, user *identityentity.User) {
	if r.cache == nil {
		return
	}

	if err := r.cache.Set(ctx, mapper.UserToCacheSchema(user)); err != nil {
		slog.WarnContext(ctx, "cache user failed",
			slog.String("user_id", user.Id.String()),
			slog.Any("err", err),
		)
	}
}

func (r *UserRepositoryImpl) deleteCache(ctx context.Context, id string) {
	if r.cache == nil {
		return
	}

	if err := r.cache.Delete(ctx, id); err != nil {
		slog.WarnContext(ctx, "delete user cache failed",
			slog.String("user_id", id),
			slog.Any("err", err),
		)
	}
}
