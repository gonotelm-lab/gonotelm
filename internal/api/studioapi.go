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
	artifactGroup := g.Group("/studio/artifact/:task_id")
	artifactGroup.Use(s.checkArtifactUserMiddleware)
	{
		artifactGroup.GET("/status", s.GetStudioArtifactStatus)
		artifactGroup.GET("/result", s.GetStudioArtifactResult)
		artifactGroup.POST("/delete", s.DeleteStudioArtifact)
		artifactGroup.POST("/retry", s.RetryStudioArtifactTask)
		artifactGroup.POST("/cancel", s.CancelStudioArtifactTask)
	}

	g.POST("/studio/artifact/generate", s.GenerateStudioArtifact)
}

func (s *Server) checkArtifactUserMiddleware(ctx context.Context, c *app.RequestContext) {
	taskId := c.Param("task_id")
	if taskId == "" {
		http.ErrResp(c, errors.ErrParams.Msgf("task_id is required"))
		return
	}

	tid, err := uuid.ParseString(taskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id: %s", taskId))
		return
	}

	err = s.studioLogic.CheckArtifactTaskUserId(ctx, tid)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	c.Next(ctx)
}

type ArtifactTaskIdRequest struct {
	TaskId uuid.UUID `path:"task_id,required"`
}

type GetStudioArtifactStatusResponse struct {
	TaskId string               `json:"task_id"`
	Status model.ArtifactStatus `json:"status"`
}

func (s *Server) GetStudioArtifactStatus(ctx context.Context, c *app.RequestContext) {
	var req ArtifactTaskIdRequest
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
	Kind       model.ArtifactKind   `json:"kind"`
	Status     model.ArtifactStatus `json:"status"`
	Title      string               `json:"title"`
	SourceIds  []uuid.UUID          `json:"source_ids,omitempty"`
	Timestamp  int64                `json:"timestamp"` // unix timestamp

	// content解释
	//
	// 如果contentKind=inline, 则content为inline内容
	// 如果contentKind=storage, 则contentUrl为产物的链接 需要请求这个链接才能获取产物
	Content     string                   `json:"content,omitempty"`
	ContentUrl  string                   `json:"content_url,omitempty"`
	ContentKind model.ArtifactResultKind `json:"content_kind"` // inline | storage
}

func (s *Server) GetStudioArtifactResult(ctx context.Context, c *app.RequestContext) {
	var req ArtifactTaskIdRequest
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

	http.OkResp(c, toArtifactResult(artifact))
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
	var req ArtifactTaskIdRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.studioLogic.DeleteArtifact(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) RetryStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
	var req ArtifactTaskIdRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.studioLogic.RetryArtifactTask(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) CancelStudioArtifactTask(ctx context.Context, c *app.RequestContext) {
	var req ArtifactTaskIdRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.studioLogic.CancelArtifactTask(ctx, req.TaskId)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func toArtifactResult(artifact *studiologic.Artifact) *ArtifactResult {
	return &ArtifactResult{
		NotebookId:  artifact.NotebookId.String(),
		TaskId:      artifact.Id.String(),
		Status:      artifact.Status,
		Kind:        artifact.Kind,
		Title:       artifact.Title,
		SourceIds:   artifact.SourceIds,
		Timestamp:   artifact.Timestamp,
		Content:     artifact.Content,
		ContentUrl:  artifact.ContentUrl,
		ContentKind: artifact.ResultKind,
	}
}

func toArtifactResults(artifacts []*studiologic.Artifact) []*ArtifactResult {
	results := make([]*ArtifactResult, 0, len(artifacts))
	for _, artifact := range artifacts {
		results = append(results, toArtifactResult(artifact))
	}

	return results
}
