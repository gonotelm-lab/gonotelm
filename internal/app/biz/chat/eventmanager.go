package chat

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache"
	"github.com/gonotelm-lab/gonotelm/internal/infra/cache/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"

	"github.com/bytedance/sonic"
)

var (
	ErrTaskNotFound   = errors.ErrNoRecord.Msg("chat stream task not found")
	ErrTaskNotRunning = errors.ErrParams.Msg("chat stream task not running")
)

type ChatEventManager struct {
	sc cache.ChatMessageStreamCache
}

func NewChatEventManager(cache cache.ChatMessageStreamCache) *ChatEventManager {
	return &ChatEventManager{
		sc: cache,
	}
}

type CreateTaskCommand struct {
	ChatId string
	UserId string
}

func (m *ChatEventManager) CreateTask(
	ctx context.Context,
	cmd *CreateTaskCommand,
) (*chatmodel.MessageStreamTask, error) {
	task := &schema.ChatMessageTask{
		Status:    chatmodel.MessageStreamTaskStatusRunning.String(),
		CreatedAt: time.Now().Unix(),
		ChatId:    cmd.ChatId,
		UserId:    cmd.UserId,
	}
	taskId, err := m.sc.SetTask(ctx, task)
	if err != nil {
		return nil, errors.Wrap(errors.ErrSerde, err.Error())
	}
	task.Id = taskId

	return chatmodel.NewMessageStreamTask(task)
}

func (m *ChatEventManager) GetTask(
	ctx context.Context,
	taskId string,
) (*chatmodel.MessageStreamTask, error) {
	task, err := m.sc.GetTask(ctx, taskId)
	if err != nil {
		if errors.Is(err, cache.ErrTaskNotFound) {
			return nil, ErrTaskNotFound
		}

		return nil, errors.Wrap(errors.ErrCache, err.Error())
	}

	return chatmodel.NewMessageStreamTask(task)
}

func (m *ChatEventManager) UpdateTaskStatus(
	ctx context.Context,
	taskId string,
	status chatmodel.MessageStreamTaskStatus,
	expireDuration time.Duration,
) error {
	task, err := m.sc.GetTask(ctx, taskId)
	if err != nil {
		return errors.WithMessage(err, "get task failed")
	}
	task.Status = status.String()
	task.ExpireDuration = expireDuration

	_, err = m.sc.SetTask(ctx, task)
	if err != nil {
		return errors.WithMessage(err, "set task failed")
	}

	return nil
}

func (m *ChatEventManager) AppendEvent(
	ctx context.Context,
	taskId string,
	event *chatmodel.MessageStreamEvent,
) (string, error) {
	evBytes, err := sonic.Marshal(event)
	if err != nil {
		return "", errors.Wrap(errors.ErrSerde, err.Error())
	}

	eventId, err := m.sc.AppendEventStream(ctx, taskId, &schema.ChatMessageStreamEvent{
		Data: evBytes,
	})
	if err != nil {
		return "", errors.WithMessage(err, "append event failed")
	}

	return eventId, nil
}

func (m *ChatEventManager) SetEventStreamTTL(
	ctx context.Context,
	taskId string,
	ttl time.Duration,
) error {
	err := m.sc.SetEventStreamTTL(ctx, taskId, ttl)
	if err != nil {
		return errors.WithMessage(err, "set event stream ttl failed")
	}

	return nil
}

type PullEventQueryArgs struct {
	LastId       string
	BlockTimeout time.Duration
}

// 拉取消息流事件 如果在超时过期之前没有数据 返回空数组;
// 如果任务不存在 返回错误
func (m *ChatEventManager) PullEvents(
	ctx context.Context,
	taskId string,
	args PullEventQueryArgs,
) ([]*chatmodel.MessageStreamEvent, error) {
	task, err := m.GetTask(ctx, taskId)
	if err != nil {
		return nil, err
	}

	if !task.Status.IsRunning() {
		return nil, ErrTaskNotRunning
	}

	// 验证是合法的streamId <ms>-<seq>
	parts := strings.Split(args.LastId, "-")
	if len(parts) != 2 {
		args.LastId = "0-0"
	} else {
		_, err1 := strconv.ParseInt(parts[0], 10, 64)
		_, err2 := strconv.ParseInt(parts[1], 10, 64)
		if err1 != nil || err2 != nil {
			args.LastId = "0-0"
		}
	}

	events, err := m.sc.PullEventStream(ctx,
		taskId,
		schema.PullEventStreamArgs{
			LastId: args.LastId,
			Block:  args.BlockTimeout,
		})
	if err != nil {
		if errors.Is(err, cache.ErrStreamNoData) {
			return nil, nil
		}

		return nil, errors.WithMessage(err, "pull event stream failed")
	}

	results := make([]*chatmodel.MessageStreamEvent, 0, len(events))
	for _, event := range events {
		var val chatmodel.MessageStreamEvent
		err = sonic.Unmarshal(event.Data, &val)
		if err != nil {
			slog.ErrorContext(ctx, "pull event unmarshal event failed",
				slog.String("task_id", taskId),
				slog.String("stream_id", event.StreamId()),
				slog.Any("err", err),
			)
		} else {
			val.StreamId = event.StreamId()
			results = append(results, &val)
		}
	}

	return results, nil
}
