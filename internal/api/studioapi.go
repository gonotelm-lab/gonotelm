package api

import (
	"context"

	studiologic "github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
)

func (s *Server) registerStudioRoutes(g *route.RouterGroup) {
	g.GET("/studio/artifact/:task_id/status", s.GetStudioArtifactStatus)
	g.POST("/studio/artifact/generate", s.GenerateStudioArtifact)
}

type GetStudioArtifactStatusRequest struct {
	TaskId uuid.UUID `path:"task_id,required"`
}

type GetStudioArtifactStatusResponse struct {
	TaskId string `json:"task_id"`
	Status string `json:"status"`
	// TODO add more result fields here
	Result string `json:"result"`
}

func (s *Server) GetStudioArtifactStatus(ctx context.Context, c *app.RequestContext) {
	var req GetStudioArtifactStatusRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	task, err := s.studioLogic.GetArtifactTask(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GetStudioArtifactStatusResponse{
		TaskId: task.Id.String(),
		Status: task.Status.String(),
	})

}

type GenerateStudioArtifactRequest struct {
	// json body
	NotebookId uuid.UUID          `json:"notebook_id,required"`
	Kind       model.ArtifactKind `json:"kind,required"`
	SourceIds  []uuid.UUID        `json:"source_ids,required"`
}

func (r *GenerateStudioArtifactRequest) Validate() error {
	if !r.Kind.Supported() {
		return errors.ErrParams.Msgf("invalid artifact kind: %s", r.Kind)
	}

	return nil
}

type GenerateStudioArtifactResponse struct {
	TaskId string `json:"task_id"`
}

func (s *Server) GenerateStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req GenerateStudioArtifactRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.studioLogic.GenerateArtifact(ctx,
		&studiologic.GenerateArtifactParams{
			NotebookId: req.NotebookId,
			Kind:       req.Kind,
			SourceIds:  req.SourceIds,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GenerateStudioArtifactResponse{
		TaskId: resp.String(),
	})
}
