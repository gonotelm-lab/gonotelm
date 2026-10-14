package user

import (
	"github.com/cloudwego/hertz/pkg/route"
)

// RegisterRoutes 账号资料路由，挂在鉴权组下
func (s *Server) RegisterRoutes(g *route.RouterGroup) {
	userGroup := g.Group("/user")
	{
		// GET /api/v1/user/me
		userGroup.GET("/me", s.GetMe)
		// PATCH /api/v1/user/me
		userGroup.PATCH("/me", s.UpdateMe)
		// POST /api/v1/user/me/avatar
		userGroup.POST("/me/avatar", s.UpdateAvatar)
	}
}
