package notelm

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/sse"
	"github.com/cloudwego/hertz/pkg/route"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/chat"
	chatagent "github.com/gonotelm-lab/gonotelm/internal/application/chat/agent"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerChatRoutes(g *route.RouterGroup) {
	chatIdGroup := g.Group("/chats/:id")
	{
		// GET /api/v1/chats/:id/messages
		chatIdGroup.GET("/messages", s.ListChatMessages)
		// POST /api/v1/chats/:id/messages
		chatIdGroup.POST("/messages", s.ChatCreateMessage)
		// POST /api/v1/chats/:id/stream/abort
		chatIdGroup.POST("/stream/abort", s.ChatAbortStream)
		// GET /api/v1/chats/:id/stream — SSE
		chatIdGroup.GET("/stream", middleware.SlowRequestThreshold(60*time.Second), s.GetChatStream)
		// DELETE /api/v1/chats/:id/context
		chatIdGroup.DELETE("/context", s.DeleteChatContext)
	}
}

func (s *Server) ChatCreateMessage(ctx context.Context, c *app.RequestContext) {
	var req schema.ChatCreateMessageRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}
	req.Prompt = strings.TrimRightFunc(req.Prompt, unicode.IsSpace)
	if req.Prompt == "" {
		http.ErrResp(c, errors.ErrParams.Msg("prompt is required"))
		return
	}

	result, err := s.chatCreateMessageHandler.Handle(ctx,
		&chatapp.CreateMessageCommand{
			ChatId:         req.Id,
			Prompt:         req.Prompt,
			SourceIds:      toValobjIds(req.SourceIds),
			Style:          chatagent.ChatMessageStyle(req.Style),
			AnswerLength:   chatagent.ChatMessageAnswerLength(req.AnswerLength),
			EnableThinking: req.EnableThinking,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ChatCreateMessageResponse{
		MsgId:  result.MsgId.String(),
		TaskId: result.TaskId.String(),
	})
}

func toValobjIds(ids []uuid.UUID) []valobj.Id {
	if len(ids) == 0 {
		return nil
	}

	result := make([]valobj.Id, 0, len(ids))
	for _, id := range ids {
		result = append(result, id)
	}

	return result
}

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

	if err := s.abortStreamHandler.Handle(ctx,
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

	result, err := s.getStreamHandler.Handle(ctx,
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

func (s *Server) ListChatMessages(ctx context.Context, c *app.RequestContext) {
	var req schema.ListChatMessagesRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.listMessagesHandler.Handle(ctx,
		&chatapp.ListMessagesQuery{
			ChatId: req.Id,
			Cursor: req.Cursor,
			Limit:  req.Limit,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ListChatMessagesResponse{
		Messages:   schema.ToMessages(result.Messages),
		Limit:      req.Limit,
		HasMore:    result.HasMore,
		NextCursor: result.NextCursor,
	})
}

func (s *Server) DeleteChatContext(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteChatContextRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.deleteChatContextHandler.Handle(ctx,
		&chatapp.DeleteChatContextCommand{
			ChatId: req.Id,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}
