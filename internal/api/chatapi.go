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
	chatlogic "github.com/gonotelm-lab/gonotelm/internal/app/logic/chat"
	chatmodel "github.com/gonotelm-lab/gonotelm/internal/app/model/chat"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/http/middleware"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/cloudwego/hertz/pkg/protocol/sse"
)

func (s *Server) registerChatRoutes(g *route.RouterGroup) {
	g.GET("/chat/message/list", s.ListChatMessages)
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
	req.Prompt = strings.TrimRightFunc(req.Prompt, unicode.IsSpace)
	if req.Prompt == "" {
		http.ErrResp(c, errors.ErrParams.Msg("prompt is required"))
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

type ListChatMessagesRequest struct {
	ChatId uuid.UUID `query:"chat_id,required"`
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

type ListChatMessageItemResponse struct {
	Id      string                          `json:"id"`
	ChatId  string                          `json:"chat_id"`
	Role    string                          `json:"role"`
	Content *ListChatMessageContentResponse `json:"content,omitempty"`
}

type ListChatMessageContentResponse struct {
	CreatedAt int64                        `json:"created_at"`
	Kind      string                       `json:"kind"`
	Text      *ListChatMessageTextResponse `json:"text,omitempty"`
}

type ListChatMessageTextResponse struct {
	Content string `json:"content"`
}

type ListChatMessagesResponse struct {
	Messages   []*ListChatMessageItemResponse `json:"messages"`
	Limit      int                            `json:"limit"`
	HasMore    bool                           `json:"has_more"`
	NextCursor int64                          `json:"next_cursor"`
}

func (s *Server) ListChatMessages(ctx context.Context, c *app.RequestContext) {
	var req ListChatMessagesRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.chatLogic.ListMessages(ctx,
		&chatlogic.ListMessagesParams{
			ChatId: req.ChatId,
			Cursor: req.Cursor,
			Limit:  req.Limit,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ListChatMessagesResponse{
		Messages:   toListChatMessageItemResponses(result.Messages),
		Limit:      req.Limit,
		HasMore:    result.HasMore,
		NextCursor: result.NextCursor,
	})
}

func toListChatMessageItemResponses(messages []*chatmodel.Message) []*ListChatMessageItemResponse {
	resp := make([]*ListChatMessageItemResponse, 0, len(messages))
	for _, msg := range messages {
		var content *ListChatMessageContentResponse
		if msg.Content != nil {
			content = &ListChatMessageContentResponse{
				CreatedAt: msg.Content.CreatedAt,
				Kind:      msg.Content.Kind,
			}

			if msg.Content.Text != nil {
				content.Text = &ListChatMessageTextResponse{
					Content: msg.Content.Text.Content,
				}
			}
		}

		resp = append(resp, &ListChatMessageItemResponse{
			Id:      msg.Id.String(),
			ChatId:  msg.ChatId.String(),
			Role:    msg.MsgRole.String(),
			Content: content,
		})
	}

	return resp
}
