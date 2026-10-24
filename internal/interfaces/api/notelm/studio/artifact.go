package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
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

	view, err := s.getArtifactStatusHandler.GetView(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	if view.Artifact.IsTerminal() {
		http.OkResp(c, schema.ToArtifactItem(view))
		return
	}

	http.OkResp(c, &schema.ArtifactItem{
		TaskId:      req.TaskId.String(),
		ContentKind: string(view.Artifact.ResultKind),
		Status:      string(view.Artifact.Status),
		Timestamp:   view.Artifact.CreateTime.Value(),
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
		Artifacts: schema.ToArtifactItems(resp.Artifacts),
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   resp.HasMore,
	})
}
