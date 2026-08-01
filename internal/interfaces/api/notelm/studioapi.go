package notelm

import (
	"context"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

const maxUserTipLength = 300

func (s *Server) registerStudioRoutes(g *route.RouterGroup) {
	artifactGroup := g.Group("/artifacts/:id")
	{
		// GET /api/v1/artifacts/:id
		artifactGroup.GET("", s.GetStudioArtifact)
		// GET /api/v1/artifacts/:id/status
		artifactGroup.GET("/status", s.GetStudioArtifactStatus)
		// DELETE /api/v1/artifacts/:id
		artifactGroup.DELETE("", s.DeleteStudioArtifact)
		// PATCH /api/v1/artifacts/:id
		artifactGroup.PATCH("", s.UpdateStudioArtifact)
		// POST /api/v1/artifacts/:id/retry
		artifactGroup.POST("/retry", s.RetryStudioArtifactTask)
		// POST /api/v1/artifacts/:id/cancel
		artifactGroup.POST("/cancel", s.CancelStudioArtifactTask)
		// POST /api/v1/artifacts/:id/convert
		artifactGroup.POST("/convert", s.ConvertNoteToSource)
	}
}

func (s *Server) GenerateStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req schema.GenerateArtifactRequest
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

	if req.Flashcard != nil {
		if req.Flashcard.Count != "" && !req.Flashcard.Count.Supported() {
			http.ErrResp(c, errors.ErrParams.Msgf("unsupported flashcard count: %s", req.Flashcard.Count))
			return
		}
		if req.Flashcard.Difficulty != "" && !req.Flashcard.Difficulty.Supported() {
			http.ErrResp(c, errors.ErrParams.Msgf("unsupported flashcard difficulty: %s", req.Flashcard.Difficulty))
			return
		}
	}

	if req.Quiz != nil {
		if req.Quiz.Count != "" && !req.Quiz.Count.Supported() {
			http.ErrResp(c, errors.ErrParams.Msgf("unsupported quiz count: %s", req.Quiz.Count))
			return
		}
		if req.Quiz.Difficulty != "" && !req.Quiz.Difficulty.Supported() {
			http.ErrResp(c, errors.ErrParams.Msgf("unsupported quiz difficulty: %s", req.Quiz.Difficulty))
			return
		}
	}

	resp, err := s.generateArtifactHandler.Handle(ctx,
		&artifactapp.GenerateRequest{
			NotebookId:    req.NotebookId,
			Kind:          req.Kind,
			SourceIds:     req.SourceIds,
			Mindmap:       req.Mindmap.ToPayload(),
			Report:        req.Report.ToPayload(),
			InfoGraphic:   req.InfoGraphic.ToPayload(),
			AudioOverview: req.AudioOverview.ToPayload(),
			Flashcard:     req.Flashcard.ToPayload(),
			Quiz:          req.Quiz.ToPayload(),
			DataTable:     req.DataTable.ToPayload(),
			Note:          req.Note.ToPayload(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GenerateArtifactResponse{TaskId: resp.ArtifactId.String()})
}

func validateStudioUserTips(req *schema.GenerateArtifactRequest) error {
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

	if req.Flashcard != nil && utf8.RuneCountInString(req.Flashcard.Tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("flashcard tip exceeds %d characters", maxUserTipLength)
	}

	if req.Quiz != nil && utf8.RuneCountInString(req.Quiz.Tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("quiz tip exceeds %d characters", maxUserTipLength)
	}

	if req.DataTable != nil && utf8.RuneCountInString(req.DataTable.Tip) > maxUserTipLength {
		return errors.ErrParams.Msgf("data_table tip exceeds %d characters", maxUserTipLength)
	}

	return nil
}

func (s *Server) GetStudioArtifactStatus(ctx context.Context, c *app.RequestContext) {
	var req schema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.getArtifactStatusHandler.Handle(ctx, &artifactapp.StatusRequest{ArtifactId: req.TaskId})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GetArtifactStatusResponse{
		TaskId: req.TaskId.String(),
		Status: resp.Status,
	})
}

func (s *Server) GetStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req schema.ArtifactTaskIdRequest
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
		http.OkResp(c, s.toArtifactItem(ctx, a))
		return
	}

	info, err := s.getArtifactStatusHandler.Handle(ctx, &artifactapp.StatusRequest{ArtifactId: req.TaskId})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result := schema.ArtifactItem{
		TaskId:      req.TaskId.String(),
		ContentKind: string(info.ResultKind),
		Status:      string(info.Status),
	}
	http.OkResp(c, &result)
}

func (s *Server) DeleteStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req schema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.deleteArtifactHandler.Handle(ctx, req.TaskId); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkRespNoContent(c)
}

func (s *Server) UpdateStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateArtifactRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	var target artifactapp.UpdateTarget
	switch req.Target {
	case schema.UpdateArtifactTargetTitle:
		target = artifactapp.UpdateTargetTitle
	default:
		http.ErrResp(c, errors.ErrParams.Msgf("unsupported update target: %s", req.Target))
		return
	}

	if err := s.updateArtifactHandler.Handle(ctx, &artifactapp.UpdateCommand{
		ArtifactId: req.Id,
		Target:     target,
		Title:      req.Title,
	}); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) RetryStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
	var req schema.ArtifactTaskIdRequest
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
	var req schema.ArtifactTaskIdRequest
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

func (s *Server) ConvertNoteToSource(ctx context.Context, c *app.RequestContext) {
	var req schema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.convertNoteToSourceHandler.Handle(ctx, &artifactapp.ConvertNoteToSourceCommand{
		ArtifactId: req.TaskId,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ConvertNoteToSourceResponse{
		SourceId: resp.SourceId.String(),
	})
}

func (s *Server) ListNotebookStudioArtifacts(ctx context.Context, c *app.RequestContext) {
	var req schema.ListNotebookArtifactsRequest
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

	http.OkResp(c, schema.ListNotebookArtifactsResponse{
		Artifacts: s.toArtifactItems(ctx, resp.Artifacts),
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   resp.HasMore,
	})
}

func (s *Server) toArtifactItem(ctx context.Context, a *artifactentity.Artifact) *schema.ArtifactItem {
	result := schema.ToArtifactItem(a)
	if contentURL, mime := s.getArtifactStatusHandler.AttachStorageURL(ctx, a); contentURL != "" {
		result.ContentUrl = contentURL
		if result.MimeType == "" {
			result.MimeType = mime
		}
	}
	return result
}

func (s *Server) toArtifactItems(ctx context.Context, artifacts []*artifactentity.Artifact) []*schema.ArtifactItem {
	results := make([]*schema.ArtifactItem, 0, len(artifacts))
	for _, a := range artifacts {
		results = append(results, s.toArtifactItem(ctx, a))
	}
	return results
}
