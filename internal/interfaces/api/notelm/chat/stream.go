package chat

import (
	"context"
	"log/slog"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (s *Server) ChatAbortStream(ctx context.Context, c *app.RequestContext) {
	var req schema.ChatAbortStreamRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	taskId, err := valobj.NewIdFromString(req.TaskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id: %s", req.TaskId))
		return
	}

	if err := s.abortChatStreamHandler.Handle(ctx,
		&chatapp.AbortStreamCommand{
			ChatId: req.Id,
			TaskId: taskId,
		}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

const (
	sseEventTypeMessage   = "message"
	sseEventTypeHeartbeat = "heartbeat"
)

func (s *Server) GetChatStream(ctx context.Context, c *app.RequestContext) {
	var req schema.GetChatStreamRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	taskId, err := valobj.NewIdFromString(req.TaskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id: %s", req.TaskId))
		return
	}

	result, err := s.getChatStreamHandler.Handle(ctx,
		&chatapp.GetStreamQuery{
			ChatId:      req.Id,
			TaskId:      taskId,
			LastEventId: req.LastStreamId,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	if result.StreamChan == nil {
		http.OkResp(c, "task not running")
		return
	}

	writer := sse.NewWriter(c)
consumeLoop:
	for item := range result.StreamChan {
		select {
		case <-ctx.Done():
			break consumeLoop
		default:
			var (
				data      []byte
				eventType string
			)

			if item.Heartbeat {
				data, err = sonic.Marshal(schema.NewStreamHeartbeat())
				eventType = sseEventTypeHeartbeat
			} else {
				data, err = sonic.Marshal(item.Event)
				eventType = sseEventTypeMessage
			}

			if err != nil {
				slog.ErrorContext(ctx, "marshal stream event failed",
					slog.String("task_id", req.TaskId),
					slog.Any("err", err),
				)
				continue
			}

			event := sse.NewEvent()
			event.SetData(data)
			event.SetEvent(eventType)
			writer.Write(event)
			event.Reset()
			event.Release()
		}
	}

	writer.Close()
}

func (s *Server) GetChatRunningTask(ctx context.Context, c *app.RequestContext) {
	var req schema.GetRunningTaskRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	taskId, err := s.getChatRunningTaskHandler.Handle(ctx,
		&chatapp.GetRunningTaskQuery{
			ChatId: req.Id,
		})
	if err != nil {
		if errors.Is(err, chaterrors.ErrStreamTaskNotFound) {
			http.OkResp(c, schema.GetRunningTaskResponse{TaskId: ""})
			return
		}
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GetRunningTaskResponse{
		TaskId: taskId.String(),
	})
}
