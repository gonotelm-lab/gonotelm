package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerChatRoutes(g *route.RouterGroup) {
	g.POST("/chat/message/create", s.ChatCreateMessage)
}

type ChatCreateMessageRequest struct {
	NotebookId uuid.UUID   `json:"notebook_id,required"`
	Prompt     string      `json:"prompt,required"`
	SourceIds  []uuid.UUID `json:"source_ids"`
}

type ChatCreateMessageResponse struct {
	MsgId string `json:"msg_id"`
}

func (s *Server) ChatCreateMessage(ctx context.Context, c *app.RequestContext) {
	var req ChatCreateMessageRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	msgId, err := s.chatLogic.CreateUserMessage(ctx, &logic.CreateUserMessageParams{
		NotebookId: req.NotebookId,
		Prompt:     req.Prompt,
		SourceIds:  req.SourceIds,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ChatCreateMessageResponse{
		MsgId: msgId.String(),
	})
}
