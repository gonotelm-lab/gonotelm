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

// gonotelm:user_session:id:{sha256(sessionId)}  ->  UserSession(msgpack)   TTL
// gonotelm:user_session:user:{userId}           ->  SET{sha256(sessionId)}  TTL
type UserSessionCacheImpl struct {
	rd goredis.UniversalClient
}

var _ cache.UserSessionCache = &UserSessionCacheImpl{}

func NewUserSessionCacheImpl(rd goredis.UniversalClient) *UserSessionCacheImpl {
	return &UserSessionCacheImpl{rd: rd}
}

func (c *UserSessionCacheImpl) hash(id string) string {
	sum := sha256.Sum256(pkgstring.AsBytes(id))
	return hex.EncodeToString(sum[:])
}

func (c *UserSessionCacheImpl) sessionKey(id string) string {
	return fmt.Sprintf("gonotelm:user_session:id:%s", c.hash(id))
}

func (c *UserSessionCacheImpl) userKey(userId string) string {
	return fmt.Sprintf("gonotelm:user_session:user:%s", userId)
}

func (c *UserSessionCacheImpl) Set(ctx context.Context, id string, session *schema.UserSession, ttl time.Duration) error {
	data, err := msgpack.Marshal(session)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal user session failed, err=%s", err.Error())
	}

	pipe := c.rd.TxPipeline()
	pipe.Set(ctx, c.sessionKey(id), data, ttl)
	pipe.SAdd(ctx, c.userKey(session.UserId), c.hash(id))
	pipe.Expire(ctx, c.userKey(session.UserId), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "set user session failed, err=%s", err.Error())
	}

	return nil
}

func (c *UserSessionCacheImpl) Get(ctx context.Context, id string) (*schema.UserSession, error) {
	data, err := c.rd.Get(ctx, c.sessionKey(id)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}
		return nil, errors.Wrapf(errors.ErrCache, "get user session failed, err=%s", err.Error())
	}

	session := &schema.UserSession{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(data), session); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal user session failed, err=%s", err.Error())
	}

	return session, nil
}

func (c *UserSessionCacheImpl) Delete(ctx context.Context, id string) error {
	session, err := c.Get(ctx, id)
	if err != nil && !errors.Is(err, errors.ErrNoRecord) {
		return err
	}

	pipe := c.rd.TxPipeline()
	pipe.Del(ctx, c.sessionKey(id))
	if session != nil {
		pipe.SRem(ctx, c.userKey(session.UserId), c.hash(id))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete user session failed, err=%s", err.Error())
	}

	return nil
}

func (c *UserSessionCacheImpl) DeleteByUserId(ctx context.Context, userId string) error {
	hashedIds, err := c.rd.SMembers(ctx, c.userKey(userId)).Result()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "list user sessions failed, err=%s", err.Error())
	}

	pipe := c.rd.TxPipeline()
	for _, hashedId := range hashedIds {
		pipe.Del(ctx, fmt.Sprintf("gonotelm:user_session:id:%s", hashedId))
	}
	pipe.Del(ctx, c.userKey(userId))
	if _, err := pipe.Exec(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete user sessions failed, err=%s", err.Error())
	}

	return nil
}
