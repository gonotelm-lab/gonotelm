package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic"
	"github.com/gonotelm-lab/gonotelm/internal/app/model"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerNotebooksRoutes(g *route.RouterGroup) {
	g.POST("/notebook", s.CreateNotebook)
	g.GET("/notebook/:id", s.GetNotebook)
	g.GET("/notebook/:id/source/list", s.ListNotebookSources)
	g.GET("/notebook/list", s.ListNotebooks)
	g.PUT("/notebook/:id/name", s.UpdateNotebookName)
	g.POST("/notebook/:id/chat", s.GetOrCreateNotebookChat)
}

type CreateNotebookRequest struct {
	Name string `json:"name" validate:"max=128"`
	Desc string `json:"desc" validate:"max=1024"`
}

type CreateNotebookResponse struct {
	Id string `json:"id"`
}

// Create new notebook
func (s *Server) CreateNotebook(ctx context.Context, c *app.RequestContext) {
	var req CreateNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.notebookLogic.CreateNotebook(ctx,
		&logic.CreateNotebookParams{
			Name: req.Name,
			Desc: req.Desc,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, CreateNotebookResponse{
		Id: resp.Id.String(),
	})
}

type GetNotebookRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (r *GetNotebookRequest) Validate() error {
	return nil
}

type GetNotebookResponse struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Desc        string `json:"desc"`
	SourceCount int64  `json:"source_count"`
}

type TextSourceContent struct {
	Text string `json:"text"`
}

type UrlSourceContent struct {
	Url string `json:"url"`
}

type FileSourceContent struct {
	Url      string `json:"url"` // full url link
	Filename string `json:"filename"`
	Format   string `json:"format"`
}

type NotebookSourceResponse struct {
	Id     string             `json:"id"`
	Kind   model.SourceKind   `json:"kind"`
	Status model.SourceStatus `json:"status"`
	Title  string             `json:"title"`

	Text *TextSourceContent `json:"text,omitempty"`
	Url  *UrlSourceContent  `json:"url,omitempty"`
	File *FileSourceContent `json:"file,omitempty"`
}

func (s *Server) GetNotebook(ctx context.Context, c *app.RequestContext) {
	var req GetNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	notebook, err := s.notebookLogic.GetNotebook(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GetNotebookResponse{
		Id:          notebook.Notebook.Id.String(),
		Name:        notebook.Notebook.Name,
		Desc:        notebook.Notebook.Description,
		SourceCount: notebook.SourceCount,
	})
}

type ListNotebooksSortBy string

const (
	ListNotebooksSortByLastActive ListNotebooksSortBy = "last_active"
	ListNotebooksSortByCreateTime ListNotebooksSortBy = "create_time"
)

func (s ListNotebooksSortBy) ToSortBy() logic.ListNotebooksSortBy {
	switch s {
	case ListNotebooksSortByLastActive:
		return logic.ListNotebooksSortByLastActive
	case ListNotebooksSortByCreateTime:
		return logic.ListNotebooksSortByCreateTime
	}

	return logic.ListNotebooksSortByCreateTime
}

type ListNotebooksRequest struct {
	Limit  int                 `query:"limit"   validate:"omitempty,min=1,max=100"`
	Offset int                 `query:"offset"  validate:"min=0"`
	SortBy ListNotebooksSortBy `query:"sort_by" validate:"omitempty,oneof=last_active create_time"`
}

const (
	defaultNotebooksListLimit = 20
)

func (r *ListNotebooksRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebooksListLimit
	}

	if r.SortBy == "" {
		r.SortBy = ListNotebooksSortByCreateTime
	}

	return nil
}

type ListNotebooksResponse struct {
	Notebooks []*ListNotebookItemResponse `json:"notebooks"`
	Limit     int                         `json:"limit"`
	Offset    int                         `json:"offset"`
	HasMore   bool                        `json:"has_more"`
}

type ListNotebookItemResponse struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Desc        string `json:"desc"`
	SourceCount int64  `json:"source_count"`
	UpdatedAt   int64  `json:"updated_at"` // unix ms
	CreatedAt   int64  `json:"created_at"` // unix ms
}

func (s *Server) ListNotebooks(ctx context.Context, c *app.RequestContext) {
	var req ListNotebooksRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.notebookLogic.ListNotebooks(ctx,
		&logic.ListNotebooksParams{
			Limit:  req.Limit,
			Offset: req.Offset,
			SortBy: req.SortBy.ToSortBy(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	notebooks := make([]*ListNotebookItemResponse, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		notebooks = append(notebooks, &ListNotebookItemResponse{
			Id:          notebook.Notebook.Id.String(),
			Name:        notebook.Notebook.Name,
			Desc:        notebook.Notebook.Description,
			SourceCount: notebook.SourceCount,
			UpdatedAt:   notebook.Notebook.UpdatedAt,
			CreatedAt:   notebook.Notebook.Id.UnixMilli(),
		})
	}

	http.OkResp(c, ListNotebooksResponse{
		Notebooks: notebooks,
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   result.HasMore,
	})
}

type ListNotebookSourcesRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Limit  int       `query:"limit"      validate:"omitempty,min=1,max=50"`
	Offset int       `query:"offset"     validate:"min=0"`
}

const (
	defaultNotebookSourcesLimit = 50
)

func (r *ListNotebookSourcesRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebookSourcesLimit
	}
	return nil
}

type ListNotebookSourcesResponse struct {
	Sources []*NotebookSourceResponse `json:"sources"`
	Limit   int                       `json:"limit"`
	Offset  int                       `json:"offset"`
	HasMore bool                      `json:"has_more"`
}

func (s *Server) ListNotebookSources(ctx context.Context, c *app.RequestContext) {
	var req ListNotebookSourcesRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.notebookLogic.ListNotebookSources(ctx,
		&logic.ListNotebookSourcesParams{
			NotebookId: req.Id,
			Limit:      req.Limit,
			Offset:     req.Offset,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ListNotebookSourcesResponse{
		Sources: toNotebookSourceResponses(result.Sources),
		Limit:   req.Limit,
		Offset:  req.Offset,
		HasMore: result.HasMore,
	})
}

func toNotebookSourceResponses(sources []*model.DecodedSource) []*NotebookSourceResponse {
	resp := make([]*NotebookSourceResponse, 0, len(sources))
	for _, source := range sources {
		sourceResp := NotebookSourceResponse{
			Id:     source.Id.String(),
			Kind:   source.Kind,
			Status: source.Status,
			Title:  source.Title,
		}
		if source.Kind.IsText() {
			sourceResp.Text = &TextSourceContent{
				Text: source.ContentText.Text,
			}
		}
		if source.Kind.IsUrl() {
			sourceResp.Url = &UrlSourceContent{
				Url: source.ContentUrl.Url,
			}
		}
		if source.Kind.IsFile() {
			sourceResp.File = &FileSourceContent{
				Url:      source.ContentFile.Url,
				Filename: source.ContentFile.Filename,
				Format:   source.ContentFile.Format,
			}
		}

		resp = append(resp, &sourceResp)
	}

	return resp
}

type UpdateNotebookNameRequest struct {
	Id   uuid.UUID `path:"id,required"`
	Name string    `json:"name"        validate:"min=0,max=128"`
}

func (s *Server) UpdateNotebookName(ctx context.Context, c *app.RequestContext) {
	var req UpdateNotebookNameRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.notebookLogic.UpdateNotebookName(ctx, req.Id, req.Name)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

type GetNotebookChatRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (r *GetNotebookChatRequest) Validate() error {
	return nil
}

type GetNotebookChatResponse struct {
	ChatId string `json:"chat_id"`
}

func (s *Server) GetOrCreateNotebookChat(ctx context.Context, c *app.RequestContext) {
	var req GetNotebookChatRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	chat, err := s.notebookLogic.GetOrCreateNotebookChat(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GetNotebookChatResponse{
		ChatId: chat.Id.String(),
	})
}
