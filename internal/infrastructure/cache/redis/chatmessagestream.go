package redis

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache"
	cacheerrors "github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/errors"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	goredis "github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
)

const (
	streamEventDataKey = "data"
)

type ChatMessageStreamCacheImpl struct {
	rd goredis.UniversalClient
}

func NewChatMessageStreamCacheImpl(
	rd goredis.UniversalClient,
) *ChatMessageStreamCacheImpl {
	return &ChatMessageStreamCacheImpl{
		rd: rd,
	}
}

var _ cache.ChatMessageStreamCache = &ChatMessageStreamCacheImpl{}

func streamTaskCacheKey(taskId string) string {
	// redis type string
	// key: gonotelm:stream:task:123
	// value: {task data}
	return fmt.Sprintf("gonotelm:stream:task:%s", taskId)
}

func streamTaskUserChatIdCacheKey(userId, chatId string) string {
	// redis type string
	// key: gonotelm:stream:task:user:123:chat:456
	// value: taskId
	return fmt.Sprintf("gonotelm:stream:task:user:%s:chat:%s", userId, chatId)
}

func streamTaskEventCacheKey(taskId string) string {
	// redis type stream
	// key: gonotelm:stream:task:event:123
	// value: {event data}
	return fmt.Sprintf("gonotelm:stream:task:event:%s", taskId)
}

func decodeTask(encTask string) (*schema.ChatMessageTask, error) {
	decTask := &schema.ChatMessageTask{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(encTask), decTask); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}
	return decTask, nil
}

func (c *ChatMessageStreamCacheImpl) SetTask(
	ctx context.Context,
	task *schema.ChatMessageTask,
) (string, error) {
	if task.Id == "" {
		task.Id = uuid.NewV4().String()
	}

	taskEncBytes, err := msgpack.Marshal(task)
	if err != nil {
		return task.Id, errors.Wrap(errors.ErrSerde, err.Error())
	}

	taskKey := streamTaskCacheKey(task.Id)
	taskUserChatIdKey := streamTaskUserChatIdCacheKey(task.UserId, task.ChatId)
	// we need to set task data and user data associated with the task
	_, err = c.rd.TxPipelined(ctx, func(p goredis.Pipeliner) error {
		p.Set(ctx, taskKey, taskEncBytes, task.ExpireDuration)
		p.Set(ctx, taskUserChatIdKey, task.Id, task.ExpireDuration)
		return nil
	})
	if err != nil {
		return task.Id, errors.Wrap(errors.ErrCache, err.Error())
	}

	return task.Id, nil
}

func (c *ChatMessageStreamCacheImpl) GetTask(
	ctx context.Context,
	taskId string,
) (*schema.ChatMessageTask, error) {
	encTask, err := c.rd.Get(ctx, streamTaskCacheKey(taskId)).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, cacheerrors.ErrTaskNotFound
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	decTask, err := decodeTask(encTask)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return decTask, nil
}

func (c *ChatMessageStreamCacheImpl) GetTaskByUserAndChatId(ctx context.Context, userId, chatId string) (*schema.ChatMessageTask, error) {
	taskUserChatIdKey := streamTaskUserChatIdCacheKey(userId, chatId)
	taskId, err := c.rd.Get(ctx, taskUserChatIdKey).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, cacheerrors.ErrTaskNotFound
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	if taskId == "" {
		return nil, cacheerrors.ErrTaskNotFound
	}

	return c.GetTask(ctx, taskId)
}

func (c *ChatMessageStreamCacheImpl) DeleteTask(ctx context.Context, taskId string) error {
	// get then delete
	taskKey := streamTaskCacheKey(taskId)
	var encTaskResult *goredis.StringCmd
	_, err := c.rd.TxPipelined(ctx, func(p goredis.Pipeliner) error {
		encTaskResult = p.Get(ctx, taskKey)
		p.Del(ctx, taskKey)

		return nil
	})
	// GET miss yields redis.Nil on the pipeline; still inspect the command result below
	if err != nil && !errors.Is(err, goredis.Nil) {
		return errors.Wrap(errors.ErrCache, err.Error())
	}

	if encTaskResult == nil {
		return cacheerrors.ErrTaskNotFound
	}
	encTask, err := encTaskResult.Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return cacheerrors.ErrTaskNotFound
		}
		return errors.Wrap(errors.ErrCache, err.Error())
	}

	decTask, err := decodeTask(encTask)
	if err != nil {
		return errors.Wrap(errors.ErrSerde, err.Error())
	}
	if decTask.Id != taskId {
		return errors.ErrCache.Msg("task id mismatch")
	}

	// delete task data and user data associated with the task
	taskUserChatIdKey := streamTaskUserChatIdCacheKey(decTask.UserId, decTask.ChatId)
	if err := c.rd.Del(ctx, taskUserChatIdKey).Err(); err != nil {
		return errors.Wrap(errors.ErrCache, err.Error())
	}

	return nil
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

	xaddArgs := &goredis.XAddArgs{
		Stream: streamTaskEventCacheKey(taskId),
		Values: map[string]any{
			streamEventDataKey: encEvent,
		},
	}
	if event.Id != "" {
		xaddArgs.ID = event.Id
	}

	eventId, err := c.rd.XAdd(ctx, xaddArgs).Result()
	if err != nil {
		return "", errors.Wrap(errors.ErrCache, err.Error())
	}

	return eventId, nil
}

func (c *ChatMessageStreamCacheImpl) DeleteEventStream(ctx context.Context, taskId string) error {
	if err := c.rd.Del(ctx, streamTaskEventCacheKey(taskId)).Err(); err != nil {
		return errors.Wrap(errors.ErrCache, err.Error())
	}
	return nil
}

func (c *ChatMessageStreamCacheImpl) SetEventStreamTTL(
	ctx context.Context,
	taskId string,
	ttl time.Duration,
) error {
	if err := c.rd.Expire(ctx, streamTaskEventCacheKey(taskId), ttl).Err(); err != nil {
		return errors.Wrap(errors.ErrCache, err.Error())
	}

	return nil
}

func (c *ChatMessageStreamCacheImpl) PullEventStream(
	ctx context.Context,
	taskId string,
	args schema.PullEventStreamArgs,
) ([]*schema.ChatMessageStreamEvent, error) {
	key := streamTaskEventCacheKey(taskId)

	if args.LastId == "" {
		args.LastId = "0-0"
	}

	streams, err := c.rd.XRead(ctx, &goredis.XReadArgs{
		Streams: []string{key, args.LastId},
		Block:   args.Block,
		Count:   int64(args.Count),
	}).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, cacheerrors.ErrStreamNoData
		}

		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
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

		decEvent.Id = msg.ID
		events = append(events, decEvent)
	}

	return events, nil
}
