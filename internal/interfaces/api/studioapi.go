package api

import (
	"context"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/artifact"
	studioschema "github.com/gonotelm-lab/gonotelm/internal/interfaces/api/studio"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

const maxUserTipLength = 300

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

	if req.AudioOverview != nil && !req.AudioOverview.Language.IsValid() {
		http.ErrResp(c, errors.ErrParams.Msgf("unsupported language: %s", req.AudioOverview.Language))
		return
	}

	if req.Report != nil && !req.Report.Language.IsValid() {
		http.ErrResp(c, errors.ErrParams.Msgf("unsupported language: %s", req.Report.Language))
		return
	}

	if err := validateStudioUserTips(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.generateArtifactHandler.Handle(ctx, &artifactapp.GenerateRequest{
		NotebookId:    req.NotebookId,
		Kind:          req.Kind,
		SourceIds:     req.SourceIds,
		Mindmap:       req.Mindmap.ToPayload(),
		Report:        req.Report.ToPayload(),
		InfoGraphic:   req.InfoGraphic.ToPayload(),
		AudioOverview: req.AudioOverview.ToPayload(),
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, studioschema.GenerateArtifactResponse{TaskId: resp.ArtifactId.String()})
}

func validateStudioUserTips(req *studioschema.GenerateArtifactRequest) error {
	if req.Mindmap != nil && utf8.RuneCountInString(req.Mindmap.Tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("mindmap tip exceeds %d characters", maxUserTipLength)
	}
	if req.Report != nil && utf8.RuneCountInString(req.Report.Tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("report tip exceeds %d characters", maxUserTipLength)
	}
	if req.InfoGraphic != nil && utf8.RuneCountInString(req.InfoGraphic.ExtraPrompt) > maxUserTipLength {
		return errors.ErrParams.Msgf("info_graphic prompt exceeds %d characters", maxUserTipLength)
	}
	if req.AudioOverview != nil && utf8.RuneCountInString(req.AudioOverview.Tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("audio_overview tip exceeds %d characters", maxUserTipLength)
	}
	return nil
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

	a, err := s.getArtifactStatusHandler.FindById(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	if a.IsTerminal() {
		http.OkResp(c, studioschema.ToArtifactResult(a))
		return
	}

	info, err := s.getArtifactStatusHandler.Handle(ctx, &artifactapp.StatusRequest{ArtifactId: req.TaskId})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result := studioschema.ArtifactResult{
		TaskId:      req.TaskId.String(),
		ContentKind: string(info.ResultKind),
		Status:      string(info.Status),
	}
	http.OkResp(c, &result)
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
