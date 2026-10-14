package chat

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/chat"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgctx "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// GetOrCreateNotebookChat 取笔记本的默认会话，没有就建一个
func (s *Server) GetOrCreateNotebookChat(ctx context.Context, c *app.RequestContext) {
	var req schema.GetNotebookChatRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	chat, err := s.createChatHandler.Handle(ctx,
		&chatapp.CreateChatCommand{
			NotebookId: req.Id,
			OwnerId:    pkgctx.GetUserId(ctx),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GetNotebookChatResponse{
		ChatId: chat.Id.String(),
	})
}

func (s *Server) GetChatSuggestions(ctx context.Context, c *app.RequestContext) {
	var req schema.GetChatSuggestionsRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.getChatSuggestionsHandler.Handle(ctx,
		&chatapp.ChatSuggestCommand{
			ChatId:    req.Id,
			SourceIds: toValobjIds(req.SourceIds),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GetChatSuggestionsResponse{
		Type:      string(result.Type),
		Questions: result.Questions,
	})
}
