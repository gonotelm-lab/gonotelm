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

type SignupRepositoryImpl struct {
	cache cache.SignupCache
}

var _ identityrepo.SignupRepository = &SignupRepositoryImpl{}

// NewSignupRepository 构造待验证注册的存取实现。落库形态（Redis，key 推导见
// internal/infrastructure/cache/redis/signup.go）：
//
//	gonotelm:identity:signup:{sha256(email)}           -> SignupPending(msgpack)，TTL 24h
//	    值里是 email、惰性 password_hash、ticket_hash、link_token_hash、code_hash、code_expire_at、created_at
//	gonotelm:identity:signup:link:{sha256(link_token)} -> sha256(email)，TTL 24h
//	    链接索引：明文 token 只出现在邮件里，库里存它的哈希，指向 pending
//	gonotelm:identity:signup:attempts:{sha256(email)}  -> 验证码错误次数（INCR），TTL 24h
//	gonotelm:identity:signup:cooldown:{sha256(email)}  -> 发信冷却位（SET NX），TTL 60s
//
// 验证通过、验证码超限或重发时：前两个 key 会被覆盖，重发还会删掉旧的 link 索引让旧链接立即失效；
// 建号成功后整组删除。
func NewSignupRepository(signupCache cache.SignupCache) identityrepo.SignupRepository {
	return &SignupRepositoryImpl{cache: signupCache}
}

func (r *SignupRepositoryImpl) Save(
	ctx context.Context,
	pending *identityentity.SignupPending,
	previousLinkTokenHash string,
) error {
	err := r.cache.Set(
		ctx,
		mapper.SignupToSchema(pending),
		previousLinkTokenHash,
		identityentity.SignupPendingTTL,
	)
	if err != nil {
		return errors.WithMessage(err, "failed to save signup pending")
	}

	return nil
}

func (r *SignupRepositoryImpl) GetByEmail(
	ctx context.Context,
	email string,
) (*identityentity.SignupPending, error) {
	return r.get(func() (*identityentity.SignupPending, error) {
		sch, err := r.cache.GetByEmail(ctx, email)
		if err != nil {
			return nil, err
		}

		return mapper.SignupFromSchema(sch), nil
	})
}

func (r *SignupRepositoryImpl) GetByLinkToken(
	ctx context.Context,
	linkToken string,
) (*identityentity.SignupPending, error) {
	return r.get(func() (*identityentity.SignupPending, error) {
		sch, err := r.cache.GetByLinkToken(ctx, linkToken)
		if err != nil {
			return nil, err
		}

		return mapper.SignupFromSchema(sch), nil
	})
}

func (r *SignupRepositoryImpl) DeleteByEmail(ctx context.Context, email string) error {
	if err := r.cache.DeleteByEmail(ctx, email); err != nil {
		return errors.WithMessage(err, "failed to delete signup pending")
	}

	return nil
}

func (r *SignupRepositoryImpl) IncrAttempts(ctx context.Context, email string) (int64, error) {
	attempts, err := r.cache.IncrAttempts(ctx, email, identityentity.SignupPendingTTL)
	if err != nil {
		return 0, errors.WithMessage(err, "failed to incr signup attempts")
	}

	return attempts, nil
}

func (r *SignupRepositoryImpl) StartSendCooldown(ctx context.Context, email string) error {
	ok, err := r.cache.SetCooldown(ctx, email, identityentity.SignupSendCooldown)
	if err != nil {
		return errors.WithMessage(err, "failed to start signup send cooldown")
	}
	if !ok {
		return identerr.ErrTooManyRequests
	}

	return nil
}

func (r *SignupRepositoryImpl) ClearSendCooldown(ctx context.Context, email string) error {
	if err := r.cache.DeleteCooldown(ctx, email); err != nil {
		return errors.WithMessage(err, "failed to clear signup send cooldown")
	}

	return nil
}

func (r *SignupRepositoryImpl) get(
	load func() (*identityentity.SignupPending, error),
) (*identityentity.SignupPending, error) {
	pending, err := load()
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, identerr.ErrVerificationExpired
		}

		return nil, errors.WithMessage(err, "failed to get signup pending")
	}

	return pending, nil
}
