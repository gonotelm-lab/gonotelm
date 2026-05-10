package api

import (
	"context"
	"log/slog"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

func (s *Server) registerChatRoutes(g *route.RouterGroup) {
	g.POST("/chat/message/create", s.ChatCreateMessage)
	g.POST("/chat/stream/abort", s.ChatAbortStream)
	g.GET("/chat/stream", middleware.SlowRequestThreshold(60*time.Second), s.GetChatStream) // sse api
}

type ChatCreateMessageRequest struct {
	NotebookId uuid.UUID   `json:"notebook_id,required"`
	Prompt     string      `json:"prompt,required"`
	SourceIds  []uuid.UUID `json:"source_ids"`
}

type ChatCreateMessageResponse struct {
	MsgId  string `json:"msg_id"`
	TaskId string `json:"task_id"`
}

func (s *Server) ChatCreateMessage(ctx context.Context, c *app.RequestContext) {
	var req ChatCreateMessageRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.chatLogic.CreateUserMessage(ctx,
		&chatlogic.CreateUserMessageParams{
			NotebookId: req.NotebookId,
			Prompt:     req.Prompt,
			SourceIds:  req.SourceIds,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ChatCreateMessageResponse{
		MsgId:  result.MsgId.String(),
		TaskId: result.TaskId,
	})
}

type ChatAbortStreamRequest struct {
	ChatId uuid.UUID `json:"chat_id,required"`
	TaskId string    `json:"task_id,required"`
}

func (s *Server) ChatAbortStream(ctx context.Context, c *app.RequestContext) {
	var req ChatAbortStreamRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.chatLogic.AbortStreamTask(ctx,
		&chatlogic.AbortStreamTaskParams{
			ChatId: req.ChatId,
			TaskId: req.TaskId,
		}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

type GetChatStreamRequest struct {
	ChatId       uuid.UUID `query:"chat_id,required"`
	TaskId       string    `query:"task_id,required"`
	LastStreamId string    `query:"last_stream_id"`
}

const (
	sseEventTypeMessage   = "message"
	sseEventTypeHeartbeat = "heartbeat"
)

func (s *Server) GetChatStream(ctx context.Context, c *app.RequestContext) {
	var req GetChatStreamRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.chatLogic.GetStreamTask(ctx,
		&chatlogic.GetStreamTaskParams{
			ChatId:       req.ChatId,
			TaskId:       req.TaskId,
			LastStreamId: req.LastStreamId,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	if result.StreamChan == nil {
		http.OkResp(c, "task not running")
		return
	}

	// consume channel
	writer := sse.NewWriter(c)
consumeLoop:
	for ch := range result.StreamChan {
		select {
		case <-ctx.Done(): // 客户端主动断开
			break consumeLoop
		default:
			data, err := sonic.Marshal(ch)
			if err != nil {
				slog.ErrorContext(ctx, "marshal stream event failed",
					slog.String("task_id", req.TaskId),
					slog.String("stream_id", ch.StreamId),
					slog.Any("err", err),
				)
				continue
			}

			event := sse.NewEvent()
			event.SetData(data)
			if ch.Heartbeat != "" {
				event.SetEvent(sseEventTypeHeartbeat)
			} else {
				event.SetEvent(sseEventTypeMessage)
			}
			writer.Write(event)
			event.Reset()
			event.Release()
		}
	}

	writer.Close()
}
