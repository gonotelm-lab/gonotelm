package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerStudioRoutes(g *route.RouterGroup) {
	g.GET("/studio/task/:task_id/status", s.GetStudioTaskStatus)
	g.POST("/studio/mindmap/task/create", s.CreateStudioMindmap)
}

type StudioCreateTaskRespnose struct {
	NotebookId string `json:"notebook_id"`
	TaskId     string `json:"task_id"`
}

func (s *Server) GetStudioTaskStatus(ctx context.Context, c *app.RequestContext) {
}

func (s *Server) CreateStudioMindmap(ctx context.Context, c *app.RequestContext) {
}
