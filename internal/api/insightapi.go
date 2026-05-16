package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerInsightsRoutes(g *route.RouterGroup) {
	g.POST("/insight/create", s.CreateInsight)
}

func (s *Server) CreateInsight(ctx context.Context, c *app.RequestContext) {
	
}
