package chat

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetStreamHandler struct {
	streamTaskRepo chatrepo.StreamTaskRepository
}

func NewGetStreamHandler(streamTaskRepo chatrepo.StreamTaskRepository) *GetStreamHandler {
	return &GetStreamHandler{
		streamTaskRepo: streamTaskRepo,
	}
}

type GetStreamQuery struct {
	ChatId       valobj.Id
	TaskId       valobj.Id
	LastEventId  string
}

type StreamItem struct {
	Event     *chatentity.StreamTaskEvent
	Heartbeat bool
}

type GetStreamResult struct {
	TaskId     valobj.Id
	StreamChan chan *StreamItem
}

func (h *GetStreamHandler) Handle(
	ctx context.Context,
	query *GetStreamQuery,
) (*GetStreamResult, error) {
	task, err := h.streamTaskRepo.FindById(ctx, query.TaskId)
	if err != nil {
		if errors.Is(err, chaterrors.ErrStreamTaskNotFound) {
			return nil, errors.ErrParams.Msgf("task not found, task_id=%s", query.TaskId)
		}

		return nil, errors.WithMessage(err, "find stream task failed")
	}

	userId := pkgcontext.GetUserId(ctx)
	if err := canAccessStreamTask(task, query.ChatId, userId); err != nil {
		return nil, err
	}

	result := &GetStreamResult{TaskId: query.TaskId}
	if !task.Status.IsRunning() {
		return result, nil
	}

	result.StreamChan = make(chan *StreamItem, 512)
	lastEventId := query.LastEventId

	go func() {
		defer func() {
			close(result.StreamChan)
			if p := recover(); p != nil {
				slog.ErrorContext(ctx, "get stream task loop panic",
					slog.Any("task_id", query.TaskId),
					slog.Any("err", p),
					slog.String("stack", string(debug.Stack())),
				)
			}
		}()

		lastHeartbeatAt := time.Now()
		lastMsgAt := time.Now()
		heartbeatInterval := 5 * time.Second

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			events, err := h.streamTaskRepo.BlockOnStreamEvent(ctx, query.TaskId,
				chatrepo.BlockOnStreamEventOptions{
					LastEventId: lastEventId,
					Timeout:     time.Second,
					Count:       100,
				})
			if err != nil {
				slog.ErrorContext(ctx, "pull stream events failed",
					slog.Any("task_id", query.TaskId),
					slog.Any("err", err),
				)
				return
			}

			if len(events) == 0 {
				task, taskErr := h.streamTaskRepo.FindById(ctx, query.TaskId)
				if taskErr != nil || !task.Status.IsRunning() {
					return
				}

				now := time.Now()
				if now.Sub(lastHeartbeatAt) > heartbeatInterval && now.Sub(lastMsgAt) > heartbeatInterval {
					result.StreamChan <- &StreamItem{Heartbeat: true}
					lastHeartbeatAt = now
				}

				continue
			}

			for _, event := range events {
				select {
				case <-ctx.Done():
					return
				case result.StreamChan <- &StreamItem{Event: event}:
				}

				if event.Done || event.Error != nil {
					return
				}
			}

			lastEventId = events[len(events)-1].Id
			lastMsgAt = time.Now()
		}
	}()

	return result, nil
}

func canAccessStreamTask(
	task *chatentity.StreamTask,
	chatId valobj.Id,
	userId string,
) error {
	if task.ChatId != chatId {
		return errors.ErrParams.Msgf("task chat_id mismatch, task_id=%s, chat_id=%s",
			task.Id, chatId)
	}

	if task.UserId != userId {
		return errors.ErrPermission.Msgf("task user_id mismatch, task_id=%s, user_id=%s",
			task.Id, userId)
	}

	return nil
}
