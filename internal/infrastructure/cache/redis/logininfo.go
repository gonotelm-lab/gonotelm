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

const (
	loginInfoTTL = 600 * time.Second
)

type TransientProviderLoginInfoCacheImpl struct {
	rd goredis.UniversalClient
}

func NewTransientProviderLoginInfoCacheImpl(
	rd goredis.UniversalClient,
) *TransientProviderLoginInfoCacheImpl {
	return &TransientProviderLoginInfoCacheImpl{
		rd: rd,
	}
}

var _ cache.TransientProviderLoginInfoCache = &TransientProviderLoginInfoCacheImpl{}

func (c *TransientProviderLoginInfoCacheImpl) sha256State(state string) string {
	s := sha256.New()
	s.Write(pkgstring.AsBytes(state))
	return hex.EncodeToString(s.Sum(nil))
}

func (c *TransientProviderLoginInfoCacheImpl) key(state string) string {
	return fmt.Sprintf("gonotelm:login_info:state:%s", c.sha256State(state))
}

func (c *TransientProviderLoginInfoCacheImpl) Set(ctx context.Context, state string, loginInfo *schema.TransientProviderLoginInfo) error {
	data, err := msgpack.Marshal(loginInfo)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal login info failed, err=%s", err.Error())
	}

	return c.rd.Set(ctx, c.key(state), data, loginInfoTTL).Err()
}

func (c *TransientProviderLoginInfoCacheImpl) Get(ctx context.Context, state string) (*schema.TransientProviderLoginInfo, error) {
	data, err := c.rd.Get(ctx, c.key(state)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}
		return nil, errors.Wrapf(errors.ErrCache, "get login info failed, err=%s", err.Error())
	}

	loginInfo := &schema.TransientProviderLoginInfo{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(data), loginInfo); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal login info failed, err=%s", err.Error())
	}

	return loginInfo, nil
}

func (c *TransientProviderLoginInfoCacheImpl) Delete(ctx context.Context, state string) error {
	err := c.rd.Del(ctx, c.key(state)).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "delete login info failed, err=%s", err.Error())
	}

	return nil
}
