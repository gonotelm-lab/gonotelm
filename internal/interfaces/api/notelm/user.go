package notelm

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerUserRoutes(g *route.RouterGroup) {
	userGroup := g.Group("/user")
	{
		// GET /api/v1/user/me
		userGroup.GET("/me", s.GetMe)
	}
}

func (s *Server) GetMe(ctx context.Context, c *app.RequestContext) {
	resp, err := s.getMeHandler.Handle(ctx)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.MeResponse{
		UserId:   resp.UserId,
		Nickname: resp.Nickname,
	})
}
