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

type SandboxCacheImpl struct {
	rd goredis.UniversalClient
}

func NewSandboxCacheImpl(
	rdb goredis.UniversalClient,
) *SandboxCacheImpl {
	return &SandboxCacheImpl{
		rd: rdb,
	}
}

var _ cache.SandboxCache = &SandboxCacheImpl{}

func sandboxCacheKey(userId, notebookId string) string {
	return fmt.Sprintf("gonotelm:sandbox:%s:%s", userId, notebookId)
}

func (c *SandboxCacheImpl) Set(
	ctx context.Context,
	userId, notebookId string,
	desc *schema.SandboxDescription,
	ttl time.Duration,
) error {
	encBytes, err := msgpack.Marshal(desc)
	if err != nil {
		return errors.Wrap(errors.ErrSerde, err.Error())
	}

	err = c.rd.Set(ctx, sandboxCacheKey(userId, notebookId), encBytes, ttl).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "set sandbox failed: %s", err.Error())
	}

	return nil
}

func (c *SandboxCacheImpl) Get(
	ctx context.Context,
	userId, notebookId string,
) (*schema.SandboxDescription, error) {
	encDesc, err := c.rd.Get(ctx, sandboxCacheKey(userId, notebookId)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil // 不存在，非错误
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	decDesc := &schema.SandboxDescription{}
	if err := msgpack.Unmarshal([]byte(encDesc), decDesc); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return decDesc, nil
}

func (c *SandboxCacheImpl) Delete(ctx context.Context, userId, notebookId string) error {
	err := c.rd.Del(ctx, sandboxCacheKey(userId, notebookId)).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "delete sandbox failed: %s", err.Error())
	}

	return nil
}
