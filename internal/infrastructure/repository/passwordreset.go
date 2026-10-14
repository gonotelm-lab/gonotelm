package repository

import (
	"context"

	identityentity "github.com/gonotelm-lab/gonotelm/internal/domain/identity/entity"
	identerr "github.com/gonotelm-lab/gonotelm/internal/domain/identity/errors"
	identityrepo "github.com/gonotelm-lab/gonotelm/internal/domain/identity/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/repository/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type PasswordResetRepositoryImpl struct {
	cache cache.PasswordResetCache
}

var _ identityrepo.PasswordResetRepository = &PasswordResetRepositoryImpl{}

// NewPasswordResetRepository 创建待确认找回请求的读写实现，落库形态（Redis，key 推导见
// internal/infrastructure/cache/redis/passwordreset.go）：
//
//	gonotelm:identity:passwordreset:{sha256(email)}           -> PasswordResetState(msgpack)，TTL 30m
//	    值里是 email、link_token_hash、created_at
//	gonotelm:identity:passwordreset:link:{sha256(link_token)} -> sha256(email)，TTL 30m
//	    链接索引：原始 token 只在邮件里，库里存哈希，指向 state
//	gonotelm:identity:passwordreset:cooldown:{sha256(email)}  -> 发邮件间隔标记（SET NX），TTL 60s
//
// 重发会覆盖 state 并删掉旧的 link 索引，让旧链接立即失效
// 重设成功后只删 state，链接索引等 TTL 自然过期
func NewPasswordResetRepository(
	passwordResetCache cache.PasswordResetCache,
) identityrepo.PasswordResetRepository {
	return &PasswordResetRepositoryImpl{cache: passwordResetCache}
}

func (r *PasswordResetRepositoryImpl) Save(
	ctx context.Context,
	state *identityentity.PasswordResetState,
	previousLinkTokenHash string,
) error {
	err := r.cache.Set(
		ctx,
		mapper.PasswordResetToSchema(state),
		previousLinkTokenHash,
		identityentity.PasswordResetStateTTL,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to save password reset state")
	}

	return nil
}

func (r *PasswordResetRepositoryImpl) GetByEmail(
	ctx context.Context,
	email string,
) (*identityentity.PasswordResetState, error) {
	return r.get(func() (*identityentity.PasswordResetState, error) {
		sch, err := r.cache.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}

		return mapper.PasswordResetFromSchema(sch), nil
	})
}

func (r *PasswordResetRepositoryImpl) GetByLinkToken(
	ctx context.Context,
	linkToken string,
) (*identityentity.PasswordResetState, error) {
	return r.get(func() (*identityentity.PasswordResetState, error) {
		sch, err := r.cache.GetByLinkToken(ctx, linkToken)
		if err != nil {
			return nil, err
		}

		return mapper.PasswordResetFromSchema(sch), nil
	})
}

func (r *PasswordResetRepositoryImpl) DeleteByEmail(ctx context.Context, email string) error {
	if err := r.cache.DeleteByEmail(ctx, email); err != nil {
		return errors.WithMessage(err, "failed to delete password reset state")
	}

	return nil
}

func (r *PasswordResetRepositoryImpl) StartSendCooldown(ctx context.Context, email string) error {
	ok, err := r.cache.SetCooldown(ctx, email, identityentity.PasswordResetSendCooldown)
	if err != nil {
		return errors.WithMessage(err, "failed to start password reset send cooldown")
	}
	if !ok {
		return identerr.ErrTooManyRequests
	}

	return nil
}

func (r *PasswordResetRepositoryImpl) ClearSendCooldown(ctx context.Context, email string) error {
	if err := r.cache.DeleteCooldown(ctx, email); err != nil {
		return errors.WithMessage(err, "failed to clear password reset send cooldown")
	}

	return nil
}

func (r *PasswordResetRepositoryImpl) get(
	load func() (*identityentity.PasswordResetState, error),
) (*identityentity.PasswordResetState, error) {
	state, err := load()
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identerr.ErrVerificationExpired
		}

		return nil, errors.WithMessage(err, "failed to get password reset state")
	}

	return state, nil
}
