package impl

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
)

// 采用redis list结构进行存储
type ChatMessageContextCacheImpl struct {
	rd           redis.UniversalClient
	keyGenerator func(chatId string) string
}

func NewChatMessageContextCacheImpl(
	redis redis.UniversalClient,
) *ChatMessageContextCacheImpl {
	return &ChatMessageContextCacheImpl{
		rd: redis,
		keyGenerator: func(chatId string) string {
			return fmt.Sprintf("gonotelm:chat:context:list:%s", chatId)
		},
	}
}

var _ cache.ChatContextMessageCache = &ChatMessageContextCacheImpl{}

func (c *ChatMessageContextCacheImpl) Append(
	ctx context.Context,
	chatId string,
	messages []*schema.ChatContextMessage,
) error {
	if len(messages) == 0 {
		return nil
	}

	encMsgs := make([]any, 0, len(messages))
	for idx, msg := range messages {
		if msg == nil {
			return errors.ErrParams.Msgf("chat context message at idx=%d is nil", idx)
		}

		encMsg, err := msgpack.Marshal(msg)
		if err != nil {
			return errors.Wrapf(errors.ErrSerde,
				"marshal chat context message at idx=%d failed: %s", idx, err.Error())
		}
		encMsgs = append(encMsgs, encMsg)
	}

	err := c.rd.RPush(ctx, c.keyGenerator(chatId), encMsgs...).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache,
			"append chat context message failed: %s", err.Error())
	}

	return nil
}

func (c *ChatMessageContextCacheImpl) Destroy(
	ctx context.Context,
	chatId string,
) error {
	err := c.rd.Del(ctx, c.keyGenerator(chatId)).Err()
	if err != nil {
		return errors.Wrapf(errors.ErrCache, "delete chat context message failed: %s", err.Error())
	}

	return nil
}

func (c *ChatMessageContextCacheImpl) ListAll(
	ctx context.Context,
	chatId string,
) ([]*schema.ChatContextMessage, error) {
	list, err := c.rd.LRange(ctx, c.keyGenerator(chatId), 0, -1).Result()
	if err != nil {
		return nil, errors.Wrapf(errors.ErrCache, "list chat context message err: %s", err.Error())
	}

	results := make([]*schema.ChatContextMessage, 0, len(list))
	for _, item := range list {
		var message schema.ChatContextMessage
		err := msgpack.Unmarshal([]byte(item), &message)
		if err != nil {
			return nil, errors.Wrapf(errors.ErrSerde,
				"unmarshal chat context message err: %s", err.Error())
		}
		results = append(results, &message)
	}

	return results, nil
}

func (c *ChatMessageContextCacheImpl) Override(
	ctx context.Context,
	chatId string,
	messages []*schema.ChatContextMessage,
) error {
	key := c.keyGenerator(chatId)

	encMsgs := make([]any, 0, len(messages))
	for idx, message := range messages {
		data, err := msgpack.Marshal(message)
		if err != nil {
			return errors.Wrapf(errors.ErrSerde,
				"marshal chat context message at idx=%d failed: %s", idx, err.Error())
		}
		encMsgs = append(encMsgs, data)
	}

	_, err := c.rd.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, key)
		if len(encMsgs) > 0 {
			pipe.RPush(ctx, key, encMsgs...)
		}
		return nil
	})
	if err != nil {
		return errors.Wrapf(errors.ErrCache,
			"override chat context message failed: %s", err.Error())
	}

	return nil
}
