package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (s *Server) GenerateStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req schema.GenerateArtifactRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
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
			Slides:        req.Slides.ToPayload(),
			VideoOverview: req.VideoOverview.ToPayload(),
			Note:          req.Note.ToPayload(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GenerateArtifactResponse{TaskId: resp.ArtifactId.String()})
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
		TaskId:    req.TaskId.String(),
		Status:    resp.Status,
		Timestamp: resp.CreatedAt.Value(),
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
		Timestamp:   info.CreatedAt.Value(),
	}
	http.OkResp(c, &result)
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
