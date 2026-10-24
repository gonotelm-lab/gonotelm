package repository

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	cacheschema "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserRepositoryImpl struct {
	userStore     database.UserStore
	identityStore database.UserIdentityStore
	transactor    database.Transactor
	cache         cache.UserCache
}

var _ identityrepo.UserRepository = &UserRepositoryImpl{}

func NewUserRepository(
	userStore database.UserStore,
	identityStore database.UserIdentityStore,
	transactor database.Transactor,
	userCache cache.UserCache,
) identityrepo.UserRepository {
	return &UserRepositoryImpl{
		userStore:     userStore,
		identityStore: identityStore,
		transactor:    transactor,
		cache:         userCache,
	}
}

func (r *UserRepositoryImpl) Create(ctx context.Context, user *identityentity.User) error {
	return r.create(ctx, mapper.UserToSchema(user))
}

func (r *UserRepositoryImpl) CreateWithPassword(
	ctx context.Context,
	user *identityentity.User,
	passwordHash string,
) error {
	return r.create(ctx, mapper.UserToSchemaWithPassword(user, passwordHash))
}

// CreateWithFirstIdentity 创建账号与挂第一个身份必须原子，避免留下没有登录方式的孤儿账号
func (r *UserRepositoryImpl) CreateWithFirstIdentity(
	ctx context.Context,
	user *identityentity.User,
	identity *identityentity.UserIdentity,
) error {
	err := r.transactor.Transaction(ctx, func(txCtx context.Context) error {
		if err := r.userStore.Create(txCtx, mapper.UserToSchema(user)); err != nil {
			return err
		}

		return r.identityStore.Create(txCtx, mapper.UserIdentityToSchema(identity))
	})
	if err != nil {
		return errors.WithMessage(err, "failed to create user with first identity")
	}

	return nil
}

func (r *UserRepositoryImpl) create(ctx context.Context, row *schema.User) error {
	if err := r.userStore.Create(ctx, row); err != nil {
		return errors.WithMessage(err, "failed to create user")
	}

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
			return nil, identerr.ErrUserNotFound
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

func (r *UserRepositoryImpl) BatchGetById(ctx context.Context, ids []valobj.Uid) ([]*identityentity.User, error) {
	if len(ids) == 0 {
		return []*identityentity.User{}, nil
	}

	rows, err := r.userStore.BatchGetById(ctx, ids)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get users by ids")
	}

	users := make([]*identityentity.User, 0, len(rows))
	for _, row := range rows {
		user, err := mapper.UserFromSchema(row)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *UserRepositoryImpl) GetByEmail(ctx context.Context, email string) (*identityentity.User, error) {
	sch, err := r.userStore.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identerr.ErrUserNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user by email")
	}

	return mapper.UserFromSchema(sch)
}

func (r *UserRepositoryImpl) GetPasswordHash(ctx context.Context, id valobj.Uid) (string, error) {
	hash, err := r.userStore.GetPasswordHash(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return "", identerr.ErrUserNotFound
		}

		return "", errors.WithMessage(err, "failed to get password hash")
	}

	return hash, nil
}

func (r *UserRepositoryImpl) UpdateProfile(ctx context.Context, user *identityentity.User) error {
	err := r.userStore.UpdateProfile(ctx, &schema.UserUpdateProfileParams{
		Id:        user.Id,
		Nickname:  user.Nickname,
		UpdatedAt: user.UpdatedAt.Value(),
	})
	if err != nil {
		return errors.WithMessage(err, "failed to update user profile")
	}

	r.deleteCache(ctx, user.Id.String())

	return nil
}

func (r *UserRepositoryImpl) UpdateAvatar(ctx context.Context, user *identityentity.User) error {
	err := r.userStore.UpdateAvatar(ctx, &schema.UserUpdateAvatarParams{
		Id:        user.Id,
		Avatar:    user.Avatar.Encode(),
		UpdatedAt: user.UpdatedAt.Value(),
	})
	if err != nil {
		return errors.WithMessage(err, "failed to update user avatar")
	}

	r.deleteCache(ctx, user.Id.String())

	return nil
}

func (r *UserRepositoryImpl) UpdatePasswordHash(ctx context.Context, id valobj.Uid, hash string) error {
	err := r.userStore.UpdatePasswordHash(ctx, &schema.UserUpdatePasswordParams{
		Id:           id,
		PasswordHash: hash,
		UpdatedAt:    valobj.NewTime().Value(),
	})
	if err != nil {
		return errors.WithMessage(err, "failed to update password hash")
	}

	// UpdatePasswordHash 不需要清缓存

	return nil
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
