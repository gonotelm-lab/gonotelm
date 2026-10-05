package repository

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identityerrors "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type UserSessionRepositoryImpl struct {
	cache cache.UserSessionCache
}

var _ identityrepo.UserSessionRepository = &UserSessionRepositoryImpl{}

func NewUserSessionRepository(cache cache.UserSessionCache) identityrepo.UserSessionRepository {
	return &UserSessionRepositoryImpl{cache: cache}
}

func (r *UserSessionRepositoryImpl) Save(ctx context.Context, session *identityentity.UserSession) error {
	// TTL 取会话当前的闲置过期时间点;滑动续期后 ExpireAt 前移,TTL 随之延长。
	ttl := time.Until(session.ExpireAt.Time())
	if ttl <= 0 {
		return errors.ErrParams.Msg("user session already expired")
	}

	err := r.cache.Set(ctx, session.Id, mapper.UserSessionToSchema(session), ttl)
	if err != nil {
		return errors.WithMessage(err, "failed to save user session")
	}

	return nil
}

func (r *UserSessionRepositoryImpl) Get(ctx context.Context, id string) (*identityentity.UserSession, error) {
	sch, err := r.cache.Get(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identityerrors.ErrUserSessionNotFound
		}

		return nil, errors.WithMessage(err, "failed to get user session")
	}

	session, err := mapper.UserSessionFromSchema(id, sch)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to map user session")
	}

	return session, nil
}

func (r *UserSessionRepositoryImpl) Delete(ctx context.Context, id string) error {
	if err := r.cache.Delete(ctx, id); err != nil {
		return errors.WithMessage(err, "failed to delete user session")
	}

	return nil
}

func (r *UserSessionRepositoryImpl) DeleteByUserId(ctx context.Context, userId valobj.Uid) error {
	if err := r.cache.DeleteByUserId(ctx, userId.String()); err != nil {
		return errors.WithMessage(err, "failed to delete user sessions")
	}

	return nil
}
