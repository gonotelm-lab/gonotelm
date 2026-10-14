package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/vmihailenco/msgpack/v5"

	goredis "github.com/redis/go-redis/v9"
)

// gonotelm:identity:signup:{sha256(email)}          -> SignupState(msgpack)
// gonotelm:identity:signup:attempts:{sha256(email)} -> 验证码错误次数(INCR)
// gonotelm:identity:signup:cooldown:{sha256(email)} -> 发送邮件冷却位(SET NX EX)
type SignupCacheImpl struct {
	rd goredis.UniversalClient
}

var _ cache.SignupCache = &SignupCacheImpl{}

func NewSignupCacheImpl(rdb goredis.UniversalClient) *SignupCacheImpl {
	return &SignupCacheImpl{rd: rdb}
}

// hashKey 邮箱等敏感值不进 key，统一 sha256 后拼进去
func hashKey(raw string) string {
	sum := sha256.Sum256(pkgstring.AsBytes(raw))

	return hex.EncodeToString(sum[:])
}

func (c *SignupCacheImpl) stateKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:%s", hashKey(email))
}

func (c *SignupCacheImpl) attemptsKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:attempts:%s", hashKey(email))
}

func (c *SignupCacheImpl) cooldownKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:cooldown:%s", hashKey(email))
}

func (c *SignupCacheImpl) Set(
	ctx context.Context,
	state *schema.SignupState,
	ttl time.Duration,
) error {
	data, err := msgpack.Marshal(state)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal signup state failed, err=%s", err.Error())
	}

	if err := c.rd.Set(ctx, c.stateKey(state.Email), data, ttl).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "set signup state failed, err=%s", err.Error())
	}

	return nil
}

func (c *SignupCacheImpl) GetByEmail(ctx context.Context, email string) (*schema.SignupState, error) {
	return c.get(ctx, c.stateKey(email))
}

func (c *SignupCacheImpl) DeleteByEmail(ctx context.Context, email string) error {
	err := c.rd.Del(ctx, c.stateKey(email), c.attemptsKey(email)).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "delete signup state failed, err=%s", err.Error())
	}

	return nil
}

// IncrAttempts 原子自增验证码错误次数，TTL 与 state 相同
func (c *SignupCacheImpl) IncrAttempts(ctx context.Context, email string, ttl time.Duration) (int64, error) {
	key := c.attemptsKey(email)

	pipe := c.rd.TxPipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, errors.Wrapf(errors.ErrCache, "incr signup attempts failed, err=%s", err.Error())
	}

	return incr.Val(), nil
}

// SetCooldown 用 SET NX EX 原子写入发邮件间隔标记，返回是否写入成功（已存在为 false）
func (c *SignupCacheImpl) SetCooldown(ctx context.Context, email string, ttl time.Duration) (bool, error) {
	ok, err := c.rd.SetNX(ctx, c.cooldownKey(email), "1", ttl).Result()
	if err != nil {
		return false, errors.Wrapf(errors.ErrCache, "set signup cooldown failed, err=%s", err.Error())
	}

	return ok, nil
}

func (c *SignupCacheImpl) DeleteCooldown(ctx context.Context, email string) error {
	if err := c.rd.Del(ctx, c.cooldownKey(email)).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete signup cooldown failed, err=%s", err.Error())
	}

	return nil
}

func (c *SignupCacheImpl) get(ctx context.Context, key string) (*schema.SignupState, error) {
	data, err := c.rd.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}

		return nil, errors.Wrapf(errors.ErrCache, "get signup state failed, err=%s", err.Error())
	}

	state := &schema.SignupState{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(data), state); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal signup state failed, err=%s", err.Error())
	}

	return state, nil
}
