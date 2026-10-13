package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

// OAuthStateRepositoryImpl 把 OAuth 握手状态写入 Redis，state 本身只在客户端
// key 用 sha256(state)，TTL 见 cache 实现
type OAuthStateRepositoryImpl struct {
	cache cache.OAuthStateCache
}

var _ repository.OAuthStateRepository = &OAuthStateRepositoryImpl{}

func NewOAuthStateRepository(stateCache cache.OAuthStateCache) repository.OAuthStateRepository {
	return &OAuthStateRepositoryImpl{cache: stateCache}
}

func (r *OAuthStateRepositoryImpl) Save(ctx context.Context, state *entity.OAuthState) error {
	if err := r.cache.Set(ctx, state.State, mapper.OAuthStateToSchema(state)); err != nil {
		return errors.WithMessage(err, "failed to save login state")
	}

	return nil
}

func (r *OAuthStateRepositoryImpl) Get(ctx context.Context, state string) (*entity.OAuthState, error) {
	sch, err := r.cache.Get(ctx, state)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get login state")
	}

	return mapper.OAuthStateFromSchema(sch), nil
}

func (r *OAuthStateRepositoryImpl) Delete(ctx context.Context, state string) error {
	if err := r.cache.Delete(ctx, state); err != nil {
		return errors.WithMessage(err, "failed to delete login state")
	}

	return nil
}
