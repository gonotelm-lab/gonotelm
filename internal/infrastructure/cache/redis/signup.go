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

// gonotelm:identity:signup:{sha256(email)}          -> SignupPending(msgpack)
// gonotelm:identity:signup:link:{sha256(token)}     -> sha256(email)
// gonotelm:identity:signup:attempts:{sha256(email)} -> 验证码错误次数(INCR)
// gonotelm:identity:signup:cooldown:{sha256(email)} -> 发信冷却位(SET NX EX)
type SignupCacheImpl struct {
	rd goredis.UniversalClient
}

var _ cache.SignupCache = &SignupCacheImpl{}

func NewSignupCacheImpl(rdb goredis.UniversalClient) *SignupCacheImpl {
	return &SignupCacheImpl{rd: rdb}
}

func hashKey(raw string) string {
	sum := sha256.Sum256(pkgstring.AsBytes(raw))

	return hex.EncodeToString(sum[:])
}

func (c *SignupCacheImpl) pendingKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:%s", hashKey(email))
}

// linkKeyByHash 直接用算好的 token 哈希组 key
func (c *SignupCacheImpl) linkKeyByHash(tokenHash string) string {
	return fmt.Sprintf("gonotelm:identity:signup:link:%s", tokenHash)
}

func (c *SignupCacheImpl) linkKeyByToken(linkToken string) string {
	return c.linkKeyByHash(hashKey(linkToken))
}

func (c *SignupCacheImpl) attemptsKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:attempts:%s", hashKey(email))
}

func (c *SignupCacheImpl) cooldownKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:cooldown:%s", hashKey(email))
}

func (c *SignupCacheImpl) Set(
	ctx context.Context,
	pending *schema.SignupPending,
	previousLinkTokenHash string,
	ttl time.Duration,
) error {
	data, err := msgpack.Marshal(pending)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal signup pending failed, err=%s", err.Error())
	}

	pipe := c.rd.TxPipeline()
	pipe.Set(ctx, c.pendingKey(pending.Email), data, ttl)
	pipe.Set(ctx, c.linkKeyByHash(pending.LinkTokenHash), hashKey(pending.Email), ttl)
	if previousLinkTokenHash != "" && previousLinkTokenHash != pending.LinkTokenHash {
		// 重发后旧链接立即失效
		pipe.Del(ctx, c.linkKeyByHash(previousLinkTokenHash))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "set signup pending failed, err=%s", err.Error())
	}

	return nil
}

func (c *SignupCacheImpl) GetByEmail(ctx context.Context, email string) (*schema.SignupPending, error) {
	return c.get(ctx, c.pendingKey(email))
}

func (c *SignupCacheImpl) GetByLinkToken(ctx context.Context, linkToken string) (*schema.SignupPending, error) {
	emailHash, err := c.rd.Get(ctx, c.linkKeyByToken(linkToken)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}

		return nil, errors.Wrapf(errors.ErrCache, "get signup link failed, err=%s", err.Error())
	}

	return c.get(ctx, fmt.Sprintf("gonotelm:identity:signup:%s", emailHash))
}

func (c *SignupCacheImpl) DeleteByEmail(ctx context.Context, email string) error {
	err := c.rd.Del(ctx, c.pendingKey(email), c.attemptsKey(email)).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "delete signup pending failed, err=%s", err.Error())
	}

	return nil
}

// IncrAttempts 原子自增验证码错误次数，TTL 与 pending 相同
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

func (c *SignupCacheImpl) get(ctx context.Context, key string) (*schema.SignupPending, error) {
	data, err := c.rd.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}

		return nil, errors.Wrapf(errors.ErrCache, "get signup pending failed, err=%s", err.Error())
	}

	pending := &schema.SignupPending{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(data), pending); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal signup pending failed, err=%s", err.Error())
	}

	return pending, nil
}
