package chat

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	bizchat "github.com/gonotelm-lab/gonotelm/internal/app/biz/chat"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/internal/conf"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type AbortStreamTaskParams struct {
	ChatId uuid.UUID
	TaskId string
}

func (l *Logic) AbortStreamTask(
	ctx context.Context,
	params *AbortStreamTaskParams,
) error {
	task, err := l.eventManager.GetTask(ctx, params.TaskId)
	if err != nil {
		if errors.Is(err, bizchat.ErrTaskNotFound) {
			return errors.ErrParams.Msgf("task not found, task_id=%s", params.TaskId)
		}

		return errors.WithMessage(err, "get task failed")
	}

	userId := pkgcontext.GetUserId(ctx)
	if err := canAccessTask(task, params.ChatId, userId); err != nil {
		return err
	}

	if !task.Status.IsRunning() {
		return nil
	}

	ttl := conf.Global().Logic.Chat.GetTaskTimeout()

	if err = l.eventManager.SetEventStreamTTL(ctx, params.TaskId, ttl); err != nil {
		return errors.WithMessage(err, "set event stream ttl failed")
	}

	err = l.eventManager.UpdateTaskStatus(ctx,
		params.TaskId,
		chatmodel.MessageStreamTaskStatusAborted,
		ttl,
	)
	if err != nil {
		return errors.WithMessage(err, "update task status failed")
	}

	return nil
}

type GetStreamTaskParams struct {
	ChatId       uuid.UUID
	TaskId       string
	LastStreamId string
}

type GetStreamTaskResult struct {
	TaskId     string
	StreamChan chan *chatmodel.MessageStreamEvent
}

func (l *Logic) GetStreamTask(
	ctx context.Context,
	params *GetStreamTaskParams,
) (*GetStreamTaskResult, error) {
	task, err := l.eventManager.GetTask(ctx, params.TaskId)
	if err != nil {
		if errors.Is(err, bizchat.ErrTaskNotFound) {
			return nil, errors.ErrParams.Msgf("task not found, task_id=%s", params.TaskId)
		}

		return nil, errors.WithMessage(err, "get task failed")
	}

	userId := pkgcontext.GetUserId(ctx)
	if err := canAccessTask(task, params.ChatId, userId); err != nil {
		return nil, err
	}

	result := &GetStreamTaskResult{TaskId: task.Id}
	if !task.Status.IsRunning() {
		return result, nil // task not running, we don't need to return a stream channel
	}

	result.StreamChan = make(chan *chatmodel.MessageStreamEvent, 512)
	lastStreamId := params.LastStreamId
	go func() {
		defer func() {
			close(result.StreamChan)
			if err := recover(); err != nil {
				slog.ErrorContext(ctx, "get stream task loop panic",
					slog.String("task_id", params.TaskId),
					slog.Any("err", err),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()

		now := time.Now()
		lastHeartbeatAt := now
		heartbeatInterval := time.Second * 5
		lastMsgAt := now

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			queryArgs := bizchat.PullEventQueryArgs{
				LastId:       lastStreamId,
				BlockTimeout: time.Second,
			}
			events, err := l.eventManager.PullEvents(
				ctx,
				params.TaskId,
				queryArgs,
			)
			if err != nil {
				if !errors.Is(err, bizchat.ErrTaskNotRunning) {
					slog.ErrorContext(ctx, "event manager pull events failed",
						slog.String("task_id", params.TaskId),
						slog.Any("err", err),
					)
				}
				return
			}

			if len(events) == 0 {
				now := time.Now()
				// 长时间没有数据并且到达心跳时间 则发送一个心跳
				if now.Sub(lastHeartbeatAt) > heartbeatInterval && now.Sub(lastMsgAt) > heartbeatInterval {
					result.StreamChan <- &chatmodel.MessageStreamEvent{
						Heartbeat: chatmodel.MessageStreamEventHeartbeat,
						Timestamp: now.Unix(),
					}
					lastHeartbeatAt = now
				}

				continue
			}

			for _, event := range events {
				select {
				case <-ctx.Done():
					return
				default:
					result.StreamChan <- event
				}
			}
			lastStreamId = events[len(events)-1].StreamId
			lastMsgAt = time.Now()
		}
	}()

	return result, nil
}

func (l *Logic) isTaskAborted(ctx context.Context, taskId string) bool {
	task, err := l.eventManager.GetTask(ctx, taskId)
	if err != nil {
		if errors.Is(err, bizchat.ErrTaskNotFound) {
			return true
		}

		slog.ErrorContext(ctx, "get task failed",
			slog.String("task_id", taskId),
			slog.Any("err", err),
		)

		return false
	}

	return task.Status.IsAborted()
}

// 验证task状态
func canAccessTask(
	task *chatmodel.MessageStreamTask,
	chatId uuid.UUID,
	userId string,
) error {
	if task.ChatId != chatId.String() {
		return errors.ErrParams.Msgf("task chat_id mismatch, task_id=%s, chat_id=%s",
			task.Id, chatId.String())
	}

	if task.UserId != userId {
		return errors.ErrPermission.Msgf("task user_id mismatch, task_id=%s, user_id=%s",
			task.Id, userId)
	}

	return nil
}
