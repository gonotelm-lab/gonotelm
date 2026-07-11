package api

import (
	"context"
	"log/slog"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/schema"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/chat"
	chatagent "github.com/gonotelm-lab/gonotelm/internal/application/chat/agent"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

func (s *Server) registerChatRoutes(g *route.RouterGroup) {
	chatIdGroup := g.Group("/chat/:id")
	{
		chatIdGroup.GET("/message/list", s.ListChatMessages)
		chatIdGroup.POST("/message/create", s.ChatCreateMessage)
		chatIdGroup.POST("/stream/abort", s.ChatAbortStream)
		chatIdGroup.GET("/stream", middleware.SlowRequestThreshold(60*time.Second), s.GetChatStream) // sse api
		chatIdGroup.DELETE("/context", s.DeleteChatContext)
	}
}

type ChatCreateMessageRequest struct {
	Id             uuid.UUID   `path:"id,required"`
	Prompt         string      `json:"prompt"`
	SourceIds      []uuid.UUID `json:"source_ids"`
	EnableThinking bool        `json:"enable_thinking"`
	Style          string      `json:"style"`
	AnswerLength   string      `json:"answer_length"`
}

func (r *ChatCreateMessageRequest) Validate() error {
	if r.Style == "" {
		r.Style = string(chatagent.ChatMessageStyleDefault)
	}

	if len(r.Prompt) == 0 {
		return errors.ErrParams.Msg("prompt is required")
	}

	if !chatagent.ChatMessageStyle(r.Style).IsValid() {
		return errors.ErrParams.Msgf("invalid chat style: %s", r.Style)
	}

	if r.AnswerLength == "" {
		r.AnswerLength = string(chatagent.ChatMessageAnswerLengthDefault)
	}

	if !chatagent.ChatMessageAnswerLength(r.AnswerLength).IsValid() {
		return errors.ErrParams.Msgf("invalid chat answer length: %s", r.AnswerLength)
	}

	return nil
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

	http.OkResp(c, ChatCreateMessageResponse{
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

type ChatAbortStreamRequest struct {
	Id     uuid.UUID `path:"id,required"`
	TaskId string    `json:"task_id,required"`
}

func (s *Server) ChatAbortStream(ctx context.Context, c *app.RequestContext) {
	var req ChatAbortStreamRequest
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

type GetChatStreamRequest struct {
	Id           uuid.UUID `path:"id,required"` // chat id
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

type ListChatMessagesRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Cursor int64     `query:"cursor" validate:"min=0"`
	Limit  int       `query:"limit"  validate:"omitempty,min=1,max=100"`
}

const (
	defaultChatMessagesListLimit = 20
)

func (r *ListChatMessagesRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultChatMessagesListLimit
	}
	if r.Cursor == 0 {
		r.Cursor = math.MaxInt64
	}

	return nil
}

type ListChatMessagesResponse struct {
	Messages   []*schema.Message `json:"messages"`
	Limit      int               `json:"limit"`
	HasMore    bool              `json:"has_more"`
	NextCursor int64             `json:"next_cursor"`
}

func (s *Server) ListChatMessages(ctx context.Context, c *app.RequestContext) {
	var req ListChatMessagesRequest
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

	http.OkResp(c, ListChatMessagesResponse{
		Messages:   schema.ToMessages(result.Messages),
		Limit:      req.Limit,
		HasMore:    result.HasMore,
		NextCursor: result.NextCursor,
	})
}

type DeleteChatContextRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (s *Server) DeleteChatContext(ctx context.Context, c *app.RequestContext) {
	var req DeleteChatContextRequest
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
