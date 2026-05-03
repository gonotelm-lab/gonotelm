package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerChatRoutes(g *route.RouterGroup) {
	g.POST("/chat/completion", s.ChatCompletion)
}

type ChatCompletionRequest struct {
	NotebookId uuid.UUID   `json:"notebook_id,required"`
	Prompt     string      `json:"prompt,required"`
	SourceIds  []uuid.UUID `json:"source_ids,required"`
}

// TODO
func (s *Server) ChatCompletion(ctx context.Context, c *app.RequestContext) {
	var req ChatCompletionRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}


	// TODO 流式输出
}
