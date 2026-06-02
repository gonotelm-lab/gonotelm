package api

import (
	"context"

	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerStudioRoutes(g *route.RouterGroup) {
	g.GET("/studio/task/:task_id/status", s.GetStudioTaskStatus)
	g.POST("/studio/task/create/mindmap", s.CreateStudioMindmapTask)
}

type StudioCreateTaskRespnose struct {
	NotebookId string `json:"notebook_id"`
	TaskId     string `json:"task_id"`
}

func (s *Server) GetStudioTaskStatus(ctx context.Context, c *app.RequestContext) {
}

type CreateStudioMindmapTaskRequest struct {
	NotebookId uuid.UUID   `json:"notebook_id,required"`
	SourceIds  []uuid.UUID `json:"source_ids,required"`
}

type CreateStudioMindmapTaskResponse struct {
	Mindmap string `json:"mindmap"`
}

func (s *Server) CreateStudioMindmapTask(ctx context.Context, c *app.RequestContext) {
	var req CreateStudioMindmapTaskRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	mindmap, err := s.studioLogic.CreateMindmap(ctx, &studiologic.CreateMindmapParams{
		NotebookId: req.NotebookId,
		SourceIds:  req.SourceIds,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, CreateStudioMindmapTaskResponse{
		Mindmap: mindmap,
	})
}
