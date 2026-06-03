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
	g.GET("/studio/artifact/:artifact_id/status", s.GetStudioArtifactStatus)
	g.POST("/studio/artifact/generate", s.GenerateStudioArtifact)
}

type GetStudioArtifactStatusRequest struct {
	ArtifactId uuid.UUID `path:"artifact_id,required"`
}

type GetStudioArtifactStatusResponse struct{}

func (s *Server) GetStudioArtifactStatus(ctx context.Context, c *app.RequestContext) {
	var req GetStudioArtifactStatusRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}
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
	Mindmap string `json:"mindmap"`
}

func (s *Server) GenerateStudioArtifact(ctx context.Context, c *app.RequestContext) {
	var req GenerateStudioArtifactRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	var mindmap string
	switch req.Kind {
	case model.ArtifactKindMindmap:
		mindmap, err = s.studioLogic.CreateMindmap(ctx,
			&studiologic.CreateMindmapParams{
				NotebookId: req.NotebookId,
				SourceIds:  req.SourceIds,
			})
	}

	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GenerateStudioArtifactResponse{
		Mindmap: mindmap,
	})
}
