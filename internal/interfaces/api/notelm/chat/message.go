package chat

import (
	"context"
	"strings"
	"unicode"

	"github.com/cloudwego/hertz/pkg/app"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat"
	chatagent "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat/agent"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

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

	result, err := s.createChatMessageHandler.Handle(ctx,
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
	return append(result, ids...)
}

func (s *Server) ListChatMessages(ctx context.Context, c *app.RequestContext) {
	var req schema.ListChatMessagesRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.listChatMessagesHandler.Handle(ctx,
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

	http.OkRespNoContent(c)
}
