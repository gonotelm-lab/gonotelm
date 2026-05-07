package impl

import (
	"context"
	"fmt"

	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	pkgstring "github.com/gonotelm-lab/gonotelm/pkg/string"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/vmihailenco/msgpack/v5"
)

type ChatMessageTaskCacheImpl struct {
	rd redis.UniversalClient
}

func NewChatMessageTaskCacheImpl(
	rd redis.UniversalClient,
) *ChatMessageTaskCacheImpl {
	return &ChatMessageTaskCacheImpl{
		rd: rd,
	}
}

var _ cache.ChatMessageTaskCache = &ChatMessageTaskCacheImpl{}

func taskCacheKey(taskId string) string {
	return fmt.Sprintf("gonotelm:stream:task:%s", taskId)
}

func taskEventStreamKey(taskId string) string {
	return fmt.Sprintf("gonotelm:stream:task:event:%s", taskId)
}

func (c *ChatMessageTaskCacheImpl) CreateTask(ctx context.Context, task *schema.ChatMessageTask) (string, error) {
	if task.Id == "" {
		task.Id = uuid.New().String()
	}

	encBytes, err := msgpack.Marshal(task)
	if err != nil {
		return task.Id, errors.Wrap(errors.ErrSerde, err.Error())
	}

	key := taskCacheKey(task.Id)
	if err := c.rd.Set(ctx, key, encBytes, 0).Err(); err != nil {
		return task.Id, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return task.Id, nil
}

func (c *ChatMessageTaskCacheImpl) GetTask(ctx context.Context, taskId string) (*schema.ChatMessageTask, error) {
	encTask, err := c.rd.Get(ctx, taskId).Result()
	if err != nil {
		return nil, err
	}

	decTask := &schema.ChatMessageTask{}
	if err := msgpack.Unmarshal(pkgstring.AsBytes(encTask), decTask); err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}

	return decTask, nil
}

func (c *ChatMessageTaskCacheImpl) AppendEvent(
	ctx context.Context,
	taskId string,
	event *schema.ChatMessageTaskEvent,
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
			"data": encEvent,
		},
	}).Result()
	if err != nil {
		return "", errors.Wrap(errors.ErrSerde, err.Error())
	}

	return eventId, nil
}

func (c *ChatMessageTaskCacheImpl) DeleteTask(ctx context.Context, taskId string) error {
	if err := c.rd.Del(ctx, taskCacheKey(taskId)).Err(); err != nil {
		return errors.Wrap(errors.ErrSerde, err.Error())
	}
	return nil
}

func (c *ChatMessageTaskCacheImpl) DeleteEventStream(ctx context.Context, taskId string) error {
	if err := c.rd.Del(ctx, taskEventStreamKey(taskId)).Err(); err != nil {
		return errors.Wrap(errors.ErrSerde, err.Error())
	}
	return nil
}
