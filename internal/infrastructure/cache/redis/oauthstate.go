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

const oauthStateTTL = 600 * time.Second

// gonotelm:identity:oauth_state:{sha256(state)} -> OAuthState(msgpack)
type OAuthStateCacheImpl struct {
	rd goredis.UniversalClient
}

var _ cache.OAuthStateCache = &OAuthStateCacheImpl{}

func NewOAuthStateCacheImpl(rd goredis.UniversalClient) *OAuthStateCacheImpl {
	return &OAuthStateCacheImpl{rd: rd}
}

func (c *OAuthStateCacheImpl) sha256State(state string) string {
	sum := sha256.Sum256(pkgstring.AsBytes(state))

	return hex.EncodeToString(sum[:])
}

func (c *OAuthStateCacheImpl) key(state string) string {
	return fmt.Sprintf("gonotelm:identity:oauth_state:%s", c.sha256State(state))
}

func (c *OAuthStateCacheImpl) Set(ctx context.Context, state string, oauthState *schema.OAuthState) error {
	data, err := msgpack.Marshal(oauthState)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal login state failed, err=%s", err.Error())
	}

	if err := c.rd.Set(ctx, c.key(state), data, oauthStateTTL).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "set login state failed, err=%s", err.Error())
	}

	return nil
}

func (c *OAuthStateCacheImpl) Get(ctx context.Context, state string) (*schema.OAuthState, error) {
	data, err := c.rd.Get(ctx, c.key(state)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, errors.ErrNoRecord
		}

		return nil, errors.Wrapf(errors.ErrCache, "get login state failed, err=%s", err.Error())
	}

	oauthState := &schema.OAuthState{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(data), oauthState); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal login state failed, err=%s", err.Error())
	}

	return oauthState, nil
}

func (c *OAuthStateCacheImpl) Delete(ctx context.Context, state string) error {
	if err := c.rd.Del(ctx, c.key(state)).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete login state failed, err=%s", err.Error())
	}

	return nil
}
