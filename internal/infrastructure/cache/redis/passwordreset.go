package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/vmihailenco/msgpack/v5"

	goredis "github.com/redis/go-redis/v9"
)

// gonotelm:identity:passwordreset:{sha256(email)}          -> PasswordResetPending(msgpack)
// gonotelm:identity:passwordreset:link:{sha256(token)}     -> sha256(email)
// gonotelm:identity:passwordreset:cooldown:{sha256(email)} -> 发信冷却位(SET NX EX)
type PasswordResetCacheImpl struct {
	rd goredis.UniversalClient
}

var _ cache.PasswordResetCache = &PasswordResetCacheImpl{}

func NewPasswordResetCacheImpl(rdb goredis.UniversalClient) *PasswordResetCacheImpl {
	return &PasswordResetCacheImpl{rd: rdb}
}

func (c *PasswordResetCacheImpl) pendingKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:passwordreset:%s", hashKey(email))
}

// linkKeyByHash 直接用算好的 token 哈希组 key
func (c *PasswordResetCacheImpl) linkKeyByHash(tokenHash string) string {
	return fmt.Sprintf("gonotelm:identity:passwordreset:link:%s", tokenHash)
}

func (c *PasswordResetCacheImpl) linkKeyByToken(linkToken string) string {
	return c.linkKeyByHash(hashKey(linkToken))
}

func (c *PasswordResetCacheImpl) cooldownKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:passwordreset:cooldown:%s", hashKey(email))
}

func (c *PasswordResetCacheImpl) Set(
	ctx context.Context,
	pending *schema.PasswordResetPending,
	previousLinkTokenHash string,
	ttl time.Duration,
) error {
	data, err := msgpack.Marshal(pending)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal password reset pending failed, err=%s", err.Error())
	}

	pipe := c.rd.TxPipeline()
	pipe.Set(ctx, c.pendingKey(pending.Email), data, ttl)
	pipe.Set(ctx, c.linkKeyByHash(pending.LinkTokenHash), hashKey(pending.Email), ttl)
	if previousLinkTokenHash != "" && previousLinkTokenHash != pending.LinkTokenHash {
		// 重发后旧链接立即失效
		pipe.Del(ctx, c.linkKeyByHash(previousLinkTokenHash))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "set password reset pending failed, err=%s", err.Error())
	}

	return nil
}

func (c *PasswordResetCacheImpl) GetByEmail(
	ctx context.Context,
	email string,
) (*schema.PasswordResetPending, error) {
	return c.get(ctx, c.pendingKey(email))
}

func (c *PasswordResetCacheImpl) GetByLinkToken(
	ctx context.Context,
	linkToken string,
) (*schema.PasswordResetPending, error) {
	emailHash, err := c.rd.Get(ctx, c.linkKeyByToken(linkToken)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}

		return nil, errors.Wrapf(errors.ErrCache, "get password reset link failed, err=%s", err.Error())
	}

	return c.get(ctx, fmt.Sprintf("gonotelm:identity:passwordreset:%s", emailHash))
}

func (c *PasswordResetCacheImpl) DeleteByEmail(ctx context.Context, email string) error {
	if err := c.rd.Del(ctx, c.pendingKey(email)).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete password reset pending failed, err=%s", err.Error())
	}

	return nil
}

// SetCooldown 用 SET NX EX 原子写入发邮件间隔标记，返回是否写入成功（已存在为 false）
func (c *PasswordResetCacheImpl) SetCooldown(
	ctx context.Context,
	email string,
	ttl time.Duration,
) (bool, error) {
	ok, err := c.rd.SetNX(ctx, c.cooldownKey(email), "1", ttl).Result()
	if err != nil {
		return false, errors.Wrapf(errors.ErrCache, "set password reset cooldown failed, err=%s", err.Error())
	}

	return ok, nil
}

func (c *PasswordResetCacheImpl) DeleteCooldown(ctx context.Context, email string) error {
	if err := c.rd.Del(ctx, c.cooldownKey(email)).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete password reset cooldown failed, err=%s", err.Error())
	}

	return nil
}

func (c *PasswordResetCacheImpl) get(
	ctx context.Context,
	key string,
) (*schema.PasswordResetPending, error) {
	data, err := c.rd.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}

		return nil, errors.Wrapf(errors.ErrCache, "get password reset pending failed, err=%s", err.Error())
	}

	pending := &schema.PasswordResetPending{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(data), pending); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal password reset pending failed, err=%s", err.Error())
	}

	return pending, nil
}
