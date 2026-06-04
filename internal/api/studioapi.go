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
	g.GET("/studio/artifact/:task_id/result", s.GetStudioArtifactResult)
	g.POST("/studio/artifact/generate", s.GenerateStudioArtifact)
	g.POST("/studio/artifact/:task_id/delete", s.DeleteStudioArtifact)
	g.POST("/studio/artifact/:task_id/retry", s.RetryStudioArtifactTask)
	g.POST("/studio/artifact/:task_id/cancel", s.CancelStudioArtifactTask)
}

type GetStudioArtifactRequest struct {
	TaskId uuid.UUID `path:"task_id,required"`
}

type GetStudioArtifactStatusResponse struct {
	TaskId string               `json:"task_id"`
	Status model.ArtifactStatus `json:"status"`
}

func (s *Server) GetStudioArtifactStatus(ctx context.Context, c *app.RequestContext) {
	var req GetStudioArtifactRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	status, err := s.studioLogic.GetArtifactTaskStatus(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GetStudioArtifactStatusResponse{
		TaskId: req.TaskId.String(),
		Status: status,
	})
}

type ArtifactResult struct {
	NotebookId string               `json:"notebook_id"`
	TaskId     string               `json:"task_id"`
	Status     model.ArtifactStatus `json:"status"`

	// content解释
	//
	// 如果contentKind=inline, 则content为inline内容
	// 如果contentKind=storage, 则contentUrl为产物的链接 需要请求这个链接才能获取产物
	Content     string                   `json:"content,omitempty"`
	ContentUrl  string                   `json:"content_url,omitempty"`
	ContentKind model.ArtifactResultKind `json:"content_kind"` // inline | storage
}

func (s *Server) GetStudioArtifactResult(ctx context.Context, c *app.RequestContext) {
	var req GetStudioArtifactRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	artifact, err := s.studioLogic.GetArtifactTask(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ArtifactResult{
		NotebookId:  artifact.NotebookId.String(),
		TaskId:      req.TaskId.String(),
		Status:      artifact.Status,
		Content:     artifact.Content,
		ContentUrl:  artifact.ContentUrl,
		ContentKind: artifact.ResultKind,
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

func (s *Server) DeleteStudioArtifact(ctx context.Context, c *app.RequestContext) {
}

func (s *Server) RetryStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
}

func (s *Server) CancelStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
}
