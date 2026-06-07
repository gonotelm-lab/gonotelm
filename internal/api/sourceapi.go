package api

import (
	"context"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/app/constants"
	logic "github.com/gonotelm-lab/gonotelm/internal/app/logic/source"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
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
		sourceIdGroup.POST("/file/upload", s.UploadFileSource)
		sourceIdGroup.POST("/status", s.PollSourceStatus)       // check source processing status
		sourceIdGroup.POST("/reload", s.RetrySourcePreparation) // retry source preparation
		sourceIdGroup.DELETE("/", s.DeleteSource)
		sourceIdGroup.GET("/doc/:doc_id", s.GetSourceDoc)
		sourceIdGroup.GET("/batch/docs", s.BatchGetSourceDocs)
		sourceIdGroup.GET("/parsed/content", s.GetSourceParsedContent)
		sourceIdGroup.GET("/parsed/tree", s.GetSourceParsedTree)
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

	err = s.sourceLogic.CheckSourceUserId(ctx, sid)
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
	mk := model.SourceKind(r.Kind)
	if !mk.Supported() {
		return errors.Errorf("invalid source kind: %s", r.Kind)
	}

	switch mk {
	case model.SourceKindText:
		if r.Text == "" {
			return errors.Errorf("text content is required")
		}
		if tLen := utf8.RuneCountInString(r.Text); tLen > constants.MaxSourceTextContentLength {
			return errors.Errorf("text content is too long")
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

const maxUploadFileSizeBytes int64 = constants.MaxSourceFileSizeBytes

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

	// 是否为总结性文档片段
	IsSummary bool `json:"is_summary"`
	// 当文档为总结性文档片段时 该字段标识是从哪些文档片段总结而来
	SummarizedFrom []string `json:"summarized_from,omitempty"`
}

func toGetSourceDocResponse(
	sourceId string,
	sourceTitle string,
	doc *model.SourceDoc,
) *GetSourceDocResponse {
	if doc == nil {
		return nil
	}

	summarizedFrom := make([]string, 0, len(doc.DerivedFrom))
	for _, derivedFrom := range doc.DerivedFrom {
		summarizedFrom = append(summarizedFrom, derivedFrom.String())
	}

	resp := &GetSourceDocResponse{
		SourceId:       sourceId,
		DocId:          doc.Id,
		SourceTitle:    sourceTitle,
		Content:        doc.Content,
		IsSummary:      doc.IsDerived(),
		SummarizedFrom: summarizedFrom,
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

	result, err := s.sourceLogic.GetSourceDoc(ctx,
		&logic.GetSourceDocParams{
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
	Id  uuid.UUID `path:"id,required"` // source id
	Ids []string  `query:"ids,required"`
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

	r.Ids = slices.Unique(docIDs)
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

	result, err := s.sourceLogic.BatchGetSourceDocs(ctx,
		&logic.BatchGetSourceDocsParams{
			SourceId: req.Id,
			DocIds:   req.Ids,
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

type GetSourceParsedContentRequest struct {
	Id       uuid.UUID `path:"id,required"`
	Download bool      `query:"download,optional"`
}

type GetSourceParsedContentResponse struct {
	// one of the following fields will be present
	Content string `json:"content,omitempty"`
	Url     string `json:"url,omitempty"`
}

func (s *Server) GetSourceParsedContent(ctx context.Context, c *app.RequestContext) {
	var req GetSourceParsedContentRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.sourceLogic.GetSourceParsedContent(ctx, req.Id,
		&logic.GetSourceParsedContentParams{
			Download: req.Download,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	if resp.Content == "" && resp.Url == "" {
		// no content
		http.OkRespNoContent(c)
		return
	}

	http.OkResp(c, &GetSourceParsedContentResponse{
		Content: resp.Content,
		Url:     resp.Url,
	})
}

type GetSourceParsedTreeRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (s *Server) GetSourceParsedTree(ctx context.Context, c *app.RequestContext) {
	var req GetSourceParsedTreeRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.sourceLogic.GetSourceParsedTree(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, resp)
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

	err = s.sourceLogic.UpdateSourceTitle(ctx, req.Id, req.Title)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}
