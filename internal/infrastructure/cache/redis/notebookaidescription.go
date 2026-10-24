package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	goredis "github.com/redis/go-redis/v9"
)

type NotebookAIDescriptionCacheImpl struct {
	rd goredis.UniversalClient
}

func NewNotebookAIDescriptionCacheImpl(
	rdb goredis.UniversalClient,
) *NotebookAIDescriptionCacheImpl {
	return &NotebookAIDescriptionCacheImpl{rd: rdb}
}

var _ cache.NotebookAIDescriptionCache = &NotebookAIDescriptionCacheImpl{}

func (c *NotebookAIDescriptionCacheImpl) cacheKey(notebookId, fingerprint string) string {
	return fmt.Sprintf("gonotelm:notebook:ai_desc:%s:%s", notebookId, fingerprint)
}

func (c *NotebookAIDescriptionCacheImpl) Get(
	ctx context.Context,
	notebookId, fingerprint string,
) (string, error) {
	val, err := c.rd.Get(ctx, c.cacheKey(notebookId, fingerprint)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", nil
		}
		return "", errors.Wrap(errors.ErrCache, err.Error())
	}
	return val, nil
}

func (c *NotebookAIDescriptionCacheImpl) Set(
	ctx context.Context,
	notebookId, fingerprint, desc string,
	ttl time.Duration,
) error {
	if ttl <= 0 {
		ttl = cache.NotebookAIDescriptionTTL
	}
	if err := c.rd.Set(ctx, c.cacheKey(notebookId, fingerprint), desc, ttl).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "set notebook ai description failed: %s", err.Error())
	}
	return nil
}
