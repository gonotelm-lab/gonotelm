package api

import (
	"context"
	"net/url"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerSourcesRoutes(g *route.RouterGroup) {
	g.POST("/source", s.CreateSource)
	g.POST("/source/:id/file/upload", s.UploadFileSource)
	g.POST("/source/:id/status", s.PollSourceStatus)       // check source processing status
	g.POST("/source/:id/reload", s.RetrySourcePreparation) // retry source preparation
	g.DELETE("/source/:id", s.DeleteSource)
	g.GET("/source/:id/doc/:doc_id", s.GetSourceDoc)
}

type CreateSourceRequest struct {
	NotebookId uuid.UUID `json:"notebook_id,required"`
	Kind       string    `json:"kind,required"`

	Text string `json:"text"`
	Url  string `json:"url"`

	// internal use
	parsedUrl *url.URL
}

func (r *CreateSourceRequest) Validate() error {
	mk := model.SourceKind(r.Kind)
	if !mk.Supported() {
		return errors.Errorf("invalid source kind: %s", r.Kind)
	}

	switch mk {
	case model.SourceKindText:
		if r.Text == "" {
			return errors.Errorf("text content is required")
		}
	case model.SourceKindUrl:
		parsedUrl, err := url.ParseRequestURI(r.Url)
		if err != nil {
			return errors.Errorf("invalid url: %s", r.Url)
		}
		r.parsedUrl = parsedUrl
		// check if it is a valid http/https url
		if parsedUrl.Scheme != "http" && parsedUrl.Scheme != "https" {
			return errors.Errorf("invalid url scheme: %s", parsedUrl.Scheme)
		}
		// TODO safety issue, prevent url injection attacks
	}

	return nil
}

type CreateSourceResponse struct {
	Id string `json:"id"`
}

func (s *Server) CreateSource(ctx context.Context, c *app.RequestContext) {
	var req CreateSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.sourceLogic.CreateSource(ctx, &logic.CreateSourceParams{
		NotebookId: req.NotebookId,
		Kind:       model.SourceKind(req.Kind),
		Text:       req.Text,
		Url:        req.parsedUrl,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, CreateSourceResponse{
		Id: resp.Id.String(),
	})
}

type UploadFileSourceRequest struct {
	Id uuid.UUID `path:"id,required"`

	MimeType string `json:"mime_type,required"`
	Filename string `json:"filename,required"  validate:"max=64"`
	Size     int64  `json:"size,required"      validate:"min=1"`
	Md5      string `json:"md5,required"       validate:"md5"`
}

const maxUploadFileSizeBytes int64 = 100 * 1024 * 1024

type UploadFileSourceResponse struct {
	Url     string            `json:"url"`
	Method  string            `json:"method"`
	Forms   map[string]string `json:"forms,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (r *UploadFileSourceRequest) Validate() error {
	if !model.SupportedFileMimeType(r.MimeType) {
		return errors.ErrParams.Msgf("unsupported mime_type: %s", r.MimeType)
	}
	if r.Size > maxUploadFileSizeBytes {
		return errors.ErrParams.Msg("file size exceeds")
	}

	return nil
}

func (s *Server) UploadFileSource(ctx context.Context, c *app.RequestContext) {
	var req UploadFileSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.sourceLogic.UploadFileSource(ctx, &logic.UploadSourceParams{
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

	http.OkResp(c, &UploadFileSourceResponse{
		Url:     resp.Url,
		Method:  resp.Method,
		Forms:   resp.Forms,
		Headers: resp.Headers,
	})
}

type PollSourceStatusRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type PollSourceStatusResponse struct {
	Status model.SourceStatus `json:"status"`
}

func (s *Server) PollSourceStatus(ctx context.Context, c *app.RequestContext) {
	var req PollSourceStatusRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	status, err := s.sourceLogic.PollSourceStatus(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, &PollSourceStatusResponse{
		Status: status,
	})
}

type RetrySourcePreparationRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (s *Server) RetrySourcePreparation(ctx context.Context, c *app.RequestContext) {
	var req RetrySourcePreparationRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.sourceLogic.RetrySourcePreparation(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

type DeleteSourceRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (s *Server) DeleteSource(ctx context.Context, c *app.RequestContext) {
	var req DeleteSourceRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.sourceLogic.DeleteSource(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

type GetSourceDocRequest struct {
	Id    uuid.UUID `path:"id,required"` // source id
	DocId string    `path:"doc_id,required"`
}

type GetSourceDocResponse struct {
	SourceId    string `json:"source_id"`
	DocId       string `json:"doc_id"`
	SourceTitle string `json:"source_title"`
	Content     string `json:"content"`
}

// 获取来源的文档片段
func (s *Server) GetSourceDoc(ctx context.Context, c *app.RequestContext) {
	var req GetSourceDocRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	doc, err := s.sourceLogic.GetSourceDoc(ctx, &logic.GetSourceDocParams{
		SourceId: req.Id,
		DocId:    req.DocId,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, &GetSourceDocResponse{
		SourceId:    doc.SourceId,
		DocId:       doc.DocId,
		SourceTitle: doc.SourceTitle,
		Content:     doc.Content,
	})
}
