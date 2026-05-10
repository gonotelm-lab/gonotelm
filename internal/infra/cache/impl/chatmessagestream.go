package impl

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	streamEventDataKey = "data"
)

type ChatMessageStreamCacheImpl struct {
	rd redis.UniversalClient
}

func NewChatMessageStreamCacheImpl(
	rd redis.UniversalClient,
) *ChatMessageStreamCacheImpl {
	return &ChatMessageStreamCacheImpl{
		rd: rd,
	}
}

var _ cache.ChatMessageStreamCache = &ChatMessageStreamCacheImpl{}

func taskCacheKey(taskId string) string {
	return fmt.Sprintf("gonotelm:stream:task:%s", taskId)
}

func taskEventStreamKey(taskId string) string {
	return fmt.Sprintf("gonotelm:stream:task:event:%s", taskId)
}

func (c *ChatMessageStreamCacheImpl) SetTask(
	ctx context.Context,
	task *schema.ChatMessageTask,
) (string, error) {
	if task.Id == "" {
		task.Id = uuid.NewV4().String()
	}

	encBytes, err := msgpack.Marshal(task)
	if err != nil {
		return task.Id, errors.Wrap(errors.ErrSerde, err.Error())
	}

	key := taskCacheKey(task.Id)
	if err := c.rd.Set(ctx, key, encBytes, task.ExpireDuration).Err(); err != nil {
		return task.Id, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return task.Id, nil
}

func (c *ChatMessageStreamCacheImpl) GetTask(
	ctx context.Context,
	taskId string,
) (*schema.ChatMessageTask, error) {
	encTask, err := c.rd.Get(ctx, taskCacheKey(taskId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, cache.ErrTaskNotFound
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	decTask := &schema.ChatMessageTask{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(encTask), decTask); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return decTask, nil
}

func (c *ChatMessageStreamCacheImpl) AppendEventStream(
	ctx context.Context,
	taskId string,
	event *schema.ChatMessageStreamEvent,
) (string, error) {
	if event == nil {
		return "", errors.ErrParams.Msg("event is nil")
	}

	if event.Data == nil {
		return "", errors.ErrParams.Msg("event data is nil")
	}

	encEvent, err := msgpack.Marshal(event)
	if err != nil {
		return "", errors.Wrap(errors.ErrSerde, err.Error())
	}

	eventId, err := c.rd.XAdd(ctx, &redis.XAddArgs{
		Stream: taskEventStreamKey(taskId),
		Values: map[string]any{
			streamEventDataKey: encEvent,
		},
	}).Result()
	if err != nil {
		return "", errors.Wrap(errors.ErrCache, err.Error())
	}

	return eventId, nil
}

func (c *ChatMessageStreamCacheImpl) DeleteTask(ctx context.Context, taskId string) error {
	if err := c.rd.Del(ctx, taskCacheKey(taskId)).Err(); err != nil {
		return errors.Wrap(errors.ErrCache, err.Error())
	}
	return nil
}

func (c *ChatMessageStreamCacheImpl) DeleteEventStream(ctx context.Context, taskId string) error {
	if err := c.rd.Del(ctx, taskEventStreamKey(taskId)).Err(); err != nil {
		return errors.Wrap(errors.ErrCache, err.Error())
	}
	return nil
}

func (c *ChatMessageStreamCacheImpl) SetEventStreamTTL(
	ctx context.Context,
	taskId string,
	ttl time.Duration,
) error {
	if err := c.rd.Expire(ctx, taskEventStreamKey(taskId), ttl).Err(); err != nil {
		return errors.Wrap(errors.ErrCache, err.Error())
	}

	return nil
}

func (c *ChatMessageStreamCacheImpl) PullEventStream(
	ctx context.Context,
	taskId string,
	args schema.PullEventStreamArgs,
) ([]*schema.ChatMessageStreamEvent, error) {
	key := taskEventStreamKey(taskId)

	if args.LastId == "" {
		args.LastId = "0-0" // from start
	}

	streams, err := c.rd.XRead(ctx, &redis.XReadArgs{
		Streams: []string{key, args.LastId},
		Block:   args.Block,
		Count:   int64(args.Count),
	}).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, cache.ErrStreamNoData
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	if len(streams) == 0 {
		return nil, nil
	}

	messages := streams[0].Messages
	events := make([]*schema.ChatMessageStreamEvent, 0, len(messages))

	for _, msg := range messages {
		data := msg.Values[streamEventDataKey].(string)
		b := pkgstring.AsBytes(data)
		decEvent := &schema.ChatMessageStreamEvent{}
		if err := msgpack.Unmarshal(b, decEvent); err != nil {
			slog.ErrorContext(ctx, "unmarshal event failed",
				slog.Any("err", err),
				slog.String("task_id", taskId),
				slog.String("stream_key", key),
				slog.String("event_id", msg.ID),
			)
			continue
		}

		decEvent.SetStreamId(msg.ID)
		events = append(events, decEvent)
	}

	return events, nil
}
