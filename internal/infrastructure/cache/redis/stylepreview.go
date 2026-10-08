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

const stylePreviewCacheTTL = 24 * time.Hour

type StylePreviewCacheImpl struct {
	rd goredis.UniversalClient
}

func NewStylePreviewCacheImpl(
	rdb goredis.UniversalClient,
) *StylePreviewCacheImpl {
	return &StylePreviewCacheImpl{
		rd: rdb,
	}
}

var _ cache.StylePreviewCache = &StylePreviewCacheImpl{}

// gonotelm:artifact:style_preview:{identifier} -> StylePreview(msgpack)
func (c *StylePreviewCacheImpl) cacheKey(identifier string) string {
	return fmt.Sprintf("gonotelm:artifact:style_preview:%s", identifier)
}

func (c *StylePreviewCacheImpl) Set(
	ctx context.Context,
	identifier string,
	preview *schema.StylePreview,
) error {
	encBytes, err := msgpack.Marshal(preview)
	if err != nil {
		return errors.Wrapf(errors.ErrSerde, "marshal style preview failed, err=%s", err.Error())
	}

	if err := c.rd.Set(ctx, c.cacheKey(identifier), encBytes, stylePreviewCacheTTL).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "set style preview failed, err=%s", err.Error())
	}

	return nil
}

func (c *StylePreviewCacheImpl) SetMulti(
	ctx context.Context,
	previews []*schema.StylePreview,
) error {
	if len(previews) == 0 {
		return nil
	}

	pipe := c.rd.Pipeline()
	for _, preview := range previews {
		encBytes, err := msgpack.Marshal(preview)
		if err != nil {
			return errors.Wrapf(errors.ErrSerde, "marshal style preview failed, err=%s", err.Error())
		}
		pipe.Set(ctx, c.cacheKey(preview.Identifier), encBytes, stylePreviewCacheTTL)
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return errors.Wrapf(errors.ErrCache, "set style previews failed, err=%s", err.Error())
	}

	return nil
}

func (c *StylePreviewCacheImpl) Get(
	ctx context.Context,
	identifier string,
) (*schema.StylePreview, error) {
	encPreview, err := c.rd.Get(ctx, c.cacheKey(identifier)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, nil // 不存在，非错误
		}

		return nil, errors.Wrapf(errors.ErrCache, "get style preview failed, err=%s", err.Error())
	}

	preview := &schema.StylePreview{}
	if err := msgpack.Unmarshal([]byte(encPreview), preview); err != nil {
		return nil, errors.Wrapf(errors.ErrSerde, "unmarshal style preview failed, err=%s", err.Error())
	}

	return preview, nil
}

// GetMulti 用 pipeline 里的逐 key GET，而不是 MGET：集群模式下 MGET 会被路由到
// 首 key 所在节点，多个 key 不在同一 slot 时服务端直接以 CROSSSLOT 拒绝；
// 而 Pipeline 会按每个命令的 key 分组到各自节点并发执行
// （ClusterClient.processPipeline -> mapCmdsByNode），单机 / 哨兵 / 集群都只有一次往返。
func (c *StylePreviewCacheImpl) GetMulti(
	ctx context.Context,
	identifiers []string,
) (map[string]*schema.StylePreview, error) {
	out := make(map[string]*schema.StylePreview, len(identifiers))
	if len(identifiers) == 0 {
		return out, nil
	}

	pipe := c.rd.Pipeline()
	cmds := make(map[string]*goredis.StringCmd, len(identifiers))
	for _, identifier := range identifiers {
		cmds[identifier] = pipe.Get(ctx, c.cacheKey(identifier))
	}

	// 只要有一个 key miss，Exec 就会返回 redis.Nil，因此逐条看结果。
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, goredis.Nil) {
		return nil, errors.Wrapf(errors.ErrCache, "get style previews failed, err=%s", err.Error())
	}

	for identifier, cmd := range cmds {
		encPreview, err := cmd.Result()
		if err != nil {
			if errors.Is(err, goredis.Nil) {
				continue // 不存在，非错误
			}

			return nil, errors.Wrapf(errors.ErrCache, "get style preview failed, err=%s", err.Error())
		}

		preview := &schema.StylePreview{}
		if err := msgpack.Unmarshal([]byte(encPreview), preview); err != nil {
			// 单条脏数据不拖垮整批读，当作 miss 回源并覆盖。
			continue
		}
		out[identifier] = preview
	}

	return out, nil
}

func (c *StylePreviewCacheImpl) Delete(ctx context.Context, identifier string) error {
	if err := c.rd.Del(ctx, c.cacheKey(identifier)).Err(); err != nil {
		return errors.Wrapf(errors.ErrCache, "delete style preview failed, err=%s", err.Error())
	}

	return nil
}
