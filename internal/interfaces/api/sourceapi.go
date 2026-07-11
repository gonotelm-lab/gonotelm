package api

import (
	"context"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/schema"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/source"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/slices"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerSourcesRoutes(g *route.RouterGroup) {
	g.POST("/source", s.CreateSource)

	sourceIdGroup := g.Group("/source/:id")
	sourceIdGroup.Use(s.checkSourceUserMiddleware)
	{
		sourceIdGroup.GET("", s.GetSource)
		sourceIdGroup.DELETE("", s.DeleteSource)
		sourceIdGroup.POST("/file/upload", s.UploadFileSource)
		sourceIdGroup.POST("/status", s.PollSourceStatus)       // check source processing status
		sourceIdGroup.POST("/reload", s.RetrySourcePreparation) // retry source preparation
		sourceIdGroup.GET("/doc/:doc_id", s.GetSourceDoc)
		sourceIdGroup.GET("/batch/docs", s.BatchGetSourceDocs)
		sourceIdGroup.PUT("/title", s.UpdateSourceTitle)
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

type CreateSourceRequest struct {
	NotebookId uuid.UUID `json:"notebook_id,required"`
	Kind       string    `json:"kind,required"`

	Text string `json:"text"`
	Url  string `json:"url"`

	// internal use
	parsedUrl *url.URL
}

func (r *CreateSourceRequest) Validate() error {
	mk := sourcevo.SourceKind(r.Kind)
	if !mk.Supported() {
		return errors.Errorf("invalid source kind: %s", r.Kind)
	}

	switch mk {
	case sourcevo.SourceKindText:
		if r.Text == "" {
			return errors.Errorf("text content is required")
		}
		if tLen := utf8.RuneCountInString(r.Text); tLen > 50_000 {
			return errors.Errorf("text content is too long")
		}
	case sourcevo.SourceKindUrl:
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

	userId := pkgcontext.GetUserId(ctx)
	result, err := s.createSourceHandler.Handle(ctx, &sourceapp.CreateSourceHandleCommand{
		NotebookId: req.NotebookId,
		OwnerId:    userId,
		Kind:       sourcevo.SourceKind(req.Kind),
		Text:       req.Text,
		Url:        req.parsedUrl,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, CreateSourceResponse{
		Id: result.String(),
	})
}

type UploadFileSourceRequest struct {
	Id uuid.UUID `path:"id,required"`

	MimeType string `json:"mime_type,required"`
	Filename string `json:"filename,required"  validate:"max=64"`
	Size     int64  `json:"size,required"      validate:"min=1"`
	Md5      string `json:"md5,required"       validate:"md5"`
}

const maxUploadFileSizeBytes int64 = 100 * 1024 * 1024 // 100MB

type UploadFileSourceResponse struct {
	Url     string            `json:"url"`
	Method  string            `json:"method"`
	Forms   map[string]string `json:"forms,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (r *UploadFileSourceRequest) Validate() error {
	if !sourceentity.SupportedFileMimeType(r.MimeType) {
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

	http.OkResp(c, &UploadFileSourceResponse{
		Url:     result.Url,
		Method:  result.Method,
		Forms:   result.Forms,
		Headers: result.Headers,
	})
}

type PollSourceStatusRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type PollSourceStatusResponse struct {
	Status sourcevo.SourceStatus `json:"status"`
}

func (s *Server) PollSourceStatus(ctx context.Context, c *app.RequestContext) {
	var req PollSourceStatusRequest
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

	err = s.retrySourcePreparationHandler.Handle(ctx, req.Id)
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

	err = s.deleteSourceHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

type GetSourceDocRequest struct {
	Id    uuid.UUID `path:"id,required"` // source id
	DocId uuid.UUID `path:"doc_id,required"`
}

type SourceDocPosition struct {
	Start int `json:"start"`
	End   int `json:"end"`

	BytesStart int `json:"bytes_start"`
	BytesEnd   int `json:"bytes_end"`
}

type GetSourceDocResponse struct {
	SourceId    string `json:"source_id"`
	DocId       string `json:"doc_id"`
	SourceTitle string `json:"source_title"`
	Content     string `json:"content"`

	// 文档片段位置	rune offset 位置
	Position *SourceDocPosition `json:"position,omitempty"`
}

func toGetSourceDocResponse(
	sourceId string,
	sourceTitle string,
	doc *sourceentity.SourceDoc,
) *GetSourceDocResponse {
	if doc == nil {
		return nil
	}

	resp := &GetSourceDocResponse{
		SourceId:       sourceId,
		DocId:          doc.Id.String(),
		SourceTitle:    sourceTitle,
		Content:        doc.Content,
	}
	if doc.RunePos != nil {
		resp.Position = &SourceDocPosition{
			Start: doc.RunePos.GetStart(),
			End:   doc.RunePos.GetEnd(),
		}
	}
	if doc.BytePos != nil {
		resp.Position.BytesStart = doc.BytePos.GetStart()
		resp.Position.BytesEnd = doc.BytePos.GetEnd()
	}

	return resp
}

// 获取来源的文档片段
func (s *Server) GetSourceDoc(ctx context.Context, c *app.RequestContext) {
	var req GetSourceDocRequest
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

	http.OkResp(c, toGetSourceDocResponse(result.SourceId, result.SourceTitle, result.Doc))
}

const maxBatchGetSourceDocsCount = 50

type BatchGetSourceDocsRequest struct {
	Id     uuid.UUID   `path:"id,required"` // source id
	Ids    []string    `query:"ids,required"`
	docIds []valobj.Id
}

func (r *BatchGetSourceDocsRequest) Validate() error {
	docIDs := make([]string, 0, len(r.Ids))
	for _, item := range r.Ids {
		for docID := range strings.SplitSeq(item, ",") {
			docID = strings.TrimSpace(docID)
			if docID == "" {
				continue
			}
			docIDs = append(docIDs, docID)
		}
	}

	if len(docIDs) == 0 {
		return errors.ErrParams.Msg("ids is required")
	}
	if len(docIDs) > maxBatchGetSourceDocsCount {
		return errors.ErrParams.Msgf("ids count exceeds limit: %d", maxBatchGetSourceDocsCount)
	}

	docIDs = slices.Unique(docIDs)
	docIds := make([]valobj.Id, 0, len(docIDs))
	for _, docID := range docIDs {
		id, err := valobj.NewIdFromString(docID)
		if err != nil {
			return errors.ErrParams.Msgf("invalid doc_id: %s", docID)
		}
		docIds = append(docIds, id)
	}

	r.Ids = docIDs
	r.docIds = docIds
	return nil
}

type BatchGetSourceDocsResponse struct {
	Docs []*GetSourceDocResponse `json:"docs"`
}

// 批量获取来源文档片段
func (s *Server) BatchGetSourceDocs(ctx context.Context, c *app.RequestContext) {
	var req BatchGetSourceDocsRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.batchGetSourceDocHandler.Handle(ctx,
		&sourceapp.BatchGetSourceDocsHandleQuery{
			SourceId: req.Id,
			DocIds:   req.docIds,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	docs := make([]*GetSourceDocResponse, 0, len(result.Docs))
	for _, doc := range result.Docs {
		docs = append(docs, toGetSourceDocResponse(result.SourceId, result.SourceTitle, doc))
	}

	http.OkResp(c, &BatchGetSourceDocsResponse{
		Docs: docs,
	})
}

type GetSourceRequest struct {
	Id       uuid.UUID `path:"id,required"`
	Download bool      `query:"download,optional"`
}

func (s *Server) GetSource(ctx context.Context, c *app.RequestContext) {
	var req GetSourceRequest
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

type GetSourceParsedTreeRequest struct {
	Id uuid.UUID `path:"id,required"`
}

type UpdateSourceTitleRequest struct {
	Id    uuid.UUID `path:"id,required"`
	Title string    `json:"title" validate:"max=255"`
}

func (r *UpdateSourceTitleRequest) Validate() error {
	r.Title = strings.TrimSpace(r.Title)
	if r.Title == "" {
		return errors.ErrParams.Msg("source title is empty")
	}

	return nil
}

func (s *Server) UpdateSourceTitle(ctx context.Context, c *app.RequestContext) {
	var req UpdateSourceTitleRequest
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
