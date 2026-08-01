package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	cacheerrors "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
)

type ChatSuggestionCacheImpl struct {
	rd goredis.UniversalClient
}

func NewChatSuggestionCacheImpl(
	rdb goredis.UniversalClient,
) *ChatSuggestionCacheImpl {
	return &ChatSuggestionCacheImpl{
		rd: rdb,
	}
}

var _ cache.ChatSuggestionCache = &ChatSuggestionCacheImpl{}

func suggestionCacheKey(chatId string) string {
	return fmt.Sprintf("gonotelm:chat:suggestion:%s", chatId)
}

func (c *ChatSuggestionCacheImpl) Set(
	ctx context.Context,
	chatId string,
	suggestion *schema.ChatSuggestion,
) error {
	encBytes, err := msgpack.Marshal(suggestion)
	if err != nil {
		return errors.Wrap(errors.ErrSerde, err.Error())
	}

	err = c.rd.Set(ctx, suggestionCacheKey(chatId), encBytes, 6*time.Hour).Err() // 6 hours of expiration
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "set chat suggestion failed: %s", err.Error())
	}

	return nil
}

func (c *ChatSuggestionCacheImpl) Get(
	ctx context.Context,
	chatId string,
) (*schema.ChatSuggestion, error) {
	encSuggestion, err := c.rd.Get(ctx, suggestionCacheKey(chatId)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, cacheerrors.ErrSuggestionNotFound
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	decSuggestion := &schema.ChatSuggestion{}
	if err := msgpack.Unmarshal([]byte(encSuggestion), decSuggestion); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return decSuggestion, nil
}

func (c *ChatSuggestionCacheImpl) Delete(ctx context.Context, chatId string) error {
	err := c.rd.Del(ctx, suggestionCacheKey(chatId)).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "delete chat suggestion failed: %s", err.Error())
	}

	return nil
}
