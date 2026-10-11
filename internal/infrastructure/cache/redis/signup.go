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

// linkKeyByHash 索引键直接用已算好的 token 哈希；调用方拿明文时先 hashKey 一次。
func (c *SignupCacheImpl) linkKeyByHash(tokenHash string) string {
	return fmt.Sprintf("gonotelm:identity:signup:link:%s", tokenHash)
}

func (c *SignupCacheImpl) linkKeyByToken(linkToken string) string {
	return c.linkKeyByHash(hashKey(linkToken))
}

func (c *SignupCacheImpl) attemptsKey(email string) string {
	return fmt.Sprintf("gonotelm:identity:signup:attempts:%s", hashKey(email))
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

// IncrAttempts 原子自增验证码错误次数，并让它与 pending 同寿。
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
