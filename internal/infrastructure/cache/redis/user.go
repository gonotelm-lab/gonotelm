package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
)

const userCacheTTL = 24 * time.Hour

type UserCacheImpl struct {
	rd goredis.UniversalClient
}

var _ cache.UserCache = &UserCacheImpl{}

func NewUserCacheImpl(rdb goredis.UniversalClient) *UserCacheImpl {
	return &UserCacheImpl{rd: rdb}
}

func (c *UserCacheImpl) cacheKey(id string) string {
	return fmt.Sprintf("gonotelm:identity:user:id:%s", id)
}

func (c *UserCacheImpl) Set(ctx context.Context, user *schema.User) error {
	data, err := msgpack.Marshal(user)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal user failed, err=%s", err.Error())
	}

	if err := c.rd.Set(ctx, c.cacheKey(user.Id), data, userCacheTTL).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "set user failed, err=%s", err.Error())
	}

	return nil
}

func (c *UserCacheImpl) GetById(ctx context.Context, id string) (*schema.User, error) {
	data, err := c.rd.Get(ctx, c.cacheKey(id)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil
		}

		return nil, errors.Wrapf(errors.ErrCache, "get user failed, err=%s", err.Error())
	}

	user := &schema.User{}
	if err := msgpack.Unmarshal([]byte(data), user); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal user failed, err=%s", err.Error())
	}

	return user, nil
}

func (c *UserCacheImpl) Delete(ctx context.Context, id string) error {
	if err := c.rd.Del(ctx, c.cacheKey(id)).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete user failed, err=%s", err.Error())
	}

	return nil
}
