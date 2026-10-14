package source

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

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

	http.OkRespNoContent(c)
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
