package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/artifact"
	studioschema "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/studio"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerStudioRoutes(g *route.RouterGroup) {
	artifactGroup := g.Group("/studio/artifact/:task_id", s.checkArtifactAccess)
	{
		artifactGroup.GET("/status", s.GetStudioArtifactStatus)
		artifactGroup.GET("/result", s.GetStudioArtifactResult)
		artifactGroup.POST("/delete", s.DeleteStudioArtifact)
		artifactGroup.POST("/retry", s.RetryStudioArtifactTask)
		artifactGroup.POST("/cancel", s.CancelStudioArtifactTask)
	}
	g.POST("/studio/artifact/generate", s.GenerateStudioArtifact)
}

func (s *Server) checkArtifactAccess(ctx context.Context, c *app.RequestContext) {
	taskId := c.Param("task_id")
	tid, err := uuid.ParseString(taskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id"))
		c.Abort()
		return
	}

	err = s.getArtifactStatusHandler.CheckOwnership(ctx, tid)
	if err != nil {
		http.ErrResp(c, err)
		c.Abort()
		return
	}

	c.Next(ctx)
}

func (s *Server) GenerateStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req studioschema.GenerateArtifactRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.generateArtifactHandler.Handle(ctx, &artifactapp.GenerateRequest{
		NotebookId:    req.NotebookId,
		Kind:          req.Kind,
		SourceIds:     req.SourceIds,
		InfoGraphic:   req.InfoGraphic.ToPayload(),
		AudioOverview: req.AudioOverview.ToPayload(),
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, studioschema.GenerateArtifactResponse{TaskId: resp.ArtifactId.String()})
}

func (s *Server) GetStudioArtifactStatus(ctx context.Context, c *app.RequestContext) {
	var req studioschema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.getArtifactStatusHandler.Handle(ctx, &artifactapp.StatusRequest{ArtifactId: req.TaskId})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, studioschema.GetArtifactStatusResponse{
		TaskId: req.TaskId.String(),
		Status: resp.Status,
	})
}

func (s *Server) GetStudioArtifactResult(ctx context.Context, c *app.RequestContext) {
	var req studioschema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.getArtifactStatusHandler.Handle(ctx, &artifactapp.StatusRequest{ArtifactId: req.TaskId})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result := studioschema.GetArtifactResultResponse{
		TaskId:      req.TaskId.String(),
		Status:      resp.Status,
		Title:       resp.Title,
		ContentUrl:  resp.ContentUrl,
		MimeType:    resp.MimeType,
		ContentKind: resp.ResultKind,
	}

	if resp.ResultKind.Inline() && len(resp.Result) > 0 {
		result.Content = string(resp.Result)
	}

	http.OkResp(c, result)
}

func (s *Server) DeleteStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req studioschema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.deleteArtifactHandler.Handle(ctx, req.TaskId); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) RetryStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
	var req studioschema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.retryArtifactHandler.Handle(ctx, req.TaskId); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) CancelStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
	var req studioschema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.cancelArtifactHandler.Handle(ctx, req.TaskId); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) ListNotebookStudioArtifacts(ctx context.Context, c *app.RequestContext) {
	var req studioschema.ListNotebookArtifactsRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	const defaultLimit = 50
	if req.Limit == 0 {
		req.Limit = defaultLimit
	}

	resp, err := s.listNotebookArtifactsHandler.Handle(ctx, &artifactapp.ListRequest{
		NotebookId: req.Id,
		Limit:      req.Limit,
		Offset:     req.Offset,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, studioschema.ListNotebookArtifactsResponse{
		Artifacts: studioschema.ToArtifactResults(resp.Artifacts),
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   resp.HasMore,
	})
}
