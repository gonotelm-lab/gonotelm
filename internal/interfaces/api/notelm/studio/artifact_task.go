package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

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
