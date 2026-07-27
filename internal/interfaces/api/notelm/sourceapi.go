package notelm

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerSourcesRoutes(g *route.RouterGroup) {
	sourceIdGroup := g.Group("/sources/:id")
	sourceIdGroup.Use(s.checkSourceUserMiddleware)
	{
		// GET /api/v1/sources/:id
		sourceIdGroup.GET("", s.GetSource)
		// DELETE /api/v1/sources/:id
		sourceIdGroup.DELETE("", s.DeleteSource)
		// PATCH /api/v1/sources/:id
		sourceIdGroup.PATCH("", s.UpdateSource)
		// POST /api/v1/sources/:id/uploads
		sourceIdGroup.POST("/uploads", s.UploadFileSource)
		// POST /api/v1/sources/:id/poll — may advance uploading → preparing
		sourceIdGroup.POST("/poll", s.PollSourceStatus)
		// POST /api/v1/sources/:id/retry
		sourceIdGroup.POST("/retry", s.RetrySourcePreparation)
		// GET /api/v1/sources/:id/docs
		sourceIdGroup.GET("/docs", s.BatchGetSourceDocs)
		// GET /api/v1/sources/:id/docs/:doc_id
		sourceIdGroup.GET("/docs/:doc_id", s.GetSourceDoc)
	}
}

func (s *Server) checkSourceUserMiddleware(ctx context.Context, c *app.RequestContext) {
	sourceId := c.Param("id")
	if sourceId == "" {
		http.ErrResp(c, errors.ErrParams.Msgf("source_id is required"))
		return
	}

	sid, err := uuid.ParseString(sourceId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid source_id: %s", sourceId))
		return
	}

	err = s.checkSourceAccessHandler.Handle(ctx, sid)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	c.Next(ctx)
}

func (s *Server) CreateSource(ctx context.Context, c *app.RequestContext) {
	var req schema.CreateSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	userId := pkgcontext.GetUserId(ctx)
	result, err := s.createSourceHandler.Handle(ctx, &sourceapp.CreateSourceHandleCommand{
		NotebookId: req.NotebookId,
		OwnerId:    userId,
		Kind:       sourcevo.SourceKind(req.Kind),
		Text:       req.Text,
		Url:        req.ParsedURL(),
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.CreateSourceResponse{
		Id: result.String(),
	})
}

func (s *Server) UploadFileSource(ctx context.Context, c *app.RequestContext) {
	var req schema.UploadFileSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.presignUploadFileHandler.Handle(ctx,
		&sourceapp.PresignUploadFileHandleCommand{
			SourceId: req.Id,
			Filename: req.Filename,
			MimeType: req.MimeType,
			Size:     req.Size,
			Md5:      req.Md5,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, &schema.UploadFileSourceResponse{
		Url:     result.Url,
		Method:  result.Method,
		Forms:   result.Forms,
		Headers: result.Headers,
	})
}

func (s *Server) PollSourceStatus(ctx context.Context, c *app.RequestContext) {
	var req schema.PollSourceStatusRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	status, err := s.pollSourceStatusHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, &schema.PollSourceStatusResponse{
		Status: status,
	})
}

func (s *Server) RetrySourcePreparation(ctx context.Context, c *app.RequestContext) {
	var req schema.RetrySourcePreparationRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.retrySourcePreparationHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

func (s *Server) DeleteSource(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.deleteSourceHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

// 获取来源的文档片段
func (s *Server) GetSourceDoc(ctx context.Context, c *app.RequestContext) {
	var req schema.GetSourceDocRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.getSourceDocHandler.Handle(ctx,
		&sourceapp.GetSourceDocHandleQuery{
			SourceId: req.Id,
			DocId:    req.DocId,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ToGetSourceDocResponse(result.SourceId, result.SourceTitle, result.Doc))
}

// 批量获取来源文档片段
func (s *Server) BatchGetSourceDocs(ctx context.Context, c *app.RequestContext) {
	var req schema.BatchGetSourceDocsRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.batchGetSourceDocHandler.Handle(ctx,
		&sourceapp.BatchGetSourceDocsHandleQuery{
			SourceId: req.Id,
			DocIds:   req.DocIds(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	docs := make([]*schema.GetSourceDocResponse, 0, len(result.Docs))
	for _, doc := range result.Docs {
		docs = append(docs, schema.ToGetSourceDocResponse(result.SourceId, result.SourceTitle, doc))
	}

	http.OkResp(c, &schema.BatchGetSourceDocsResponse{
		Docs: docs,
	})
}

func (s *Server) GetSource(ctx context.Context, c *app.RequestContext) {
	var req schema.GetSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.getSourceHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ToSourceFromDomain(
		result.Source,
		result.Access.FileContentUrl,
		result.Access.ParsedContentUrl,
	))
}

func (s *Server) UpdateSource(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.updateSourceTitleHandler.Handle(ctx, req.Id, req.Title)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}
