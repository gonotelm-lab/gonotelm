package notelm

import (
	"context"

	userapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/identity/user"
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
		// PATCH /api/v1/user/me
		userGroup.PATCH("/me", s.UpdateMe)
	}
}

func (s *Server) GetMe(ctx context.Context, c *app.RequestContext) {
	resp, err := s.getMeHandler.Handle(ctx)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.MeResponse{
		UserId:      resp.UserId,
		Nickname:    resp.Nickname,
		AvatarUrl:   resp.AvatarUrl,
		CreatedAt:   resp.CreatedAt.Value(),
		UpdatedAt:   resp.UpdatedAt.Value(),
		LoginSource: resp.LoginSource.String(),
	})
}

func (s *Server) UpdateMe(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateMeRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	_, err = s.updateProfileHandler.Handle(ctx, &userapp.UpdateProfileCommand{
		Nickname: req.Nickname,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}
