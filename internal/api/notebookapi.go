package api

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	"github.com/gonotelm-lab/gonotelm/internal/api/schema"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/app/logic/studio"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notebook"
	pkgctx "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerNotebooksRoutes(g *route.RouterGroup) {
	g.POST("/notebook", s.CreateNotebook)
	g.GET("/notebook/list", s.ListNotebooks)

	notebookIdGroup := g.Group("/notebook/:id")
	notebookIdGroup.Use(s.checkNotebookUserId)
	{
		notebookIdGroup.GET("", s.GetNotebook)
		notebookIdGroup.DELETE("", s.DeleteNotebook)
		notebookIdGroup.PUT("/name", s.UpdateNotebookName)
		notebookIdGroup.POST("/chat", s.GetOrCreateNotebookChat)
		notebookIdGroup.GET("/source/list", s.ListNotebookSources)
		notebookIdGroup.GET("/studio/artifact/list", s.ListNotebookStudioArtifacts)
	}
}

func (s *Server) checkNotebookUserId(ctx context.Context, c *app.RequestContext) {
	notebookId := c.Param("id")
	nid, err := uuid.ParseString(notebookId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid notebook_id: %s", notebookId))
		return
	}

	err = s.notebookLogic.CheckNotebookUserId(ctx, nid)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	c.Next(ctx)
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

	userId := pkgctx.GetUserId(ctx)
	id, err := s.createNotebookHandler.Handle(ctx, &notebookapp.CreateNotebookHandleCommand{
		Name:    req.Name,
		Desc:    req.Desc,
		OwnerId: userId,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, CreateNotebookResponse{Id: id.String()})
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
	UpdatedAt   int64  `json:"updated_at"` // unix ms
	CreatedAt   int64  `json:"created_at"` // unix ms
}

func (s *Server) GetNotebook(ctx context.Context, c *app.RequestContext) {
	var req GetNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	notebook, err := s.getNotebookHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GetNotebookResponse{
		Id:          notebook.Id.String(),
		Name:        notebook.Name,
		Desc:        notebook.Description,
		SourceCount: notebook.SourceCount,
		UpdatedAt:   notebook.UpdateTime.Value(),
		CreatedAt:   notebook.CreateTime.Value(),
	})
}

type ListNotebooksSortBy string

const (
	ListNotebooksSortByLastActive ListNotebooksSortBy = "last_active"
	ListNotebooksSortByCreateTime ListNotebooksSortBy = "create_time"
)

func (s ListNotebooksSortBy) ToSortBy() notebookapp.SortBy {
	switch s {
	case ListNotebooksSortByLastActive:
		return notebookapp.SortByLastActive
	case ListNotebooksSortByCreateTime:
		return notebookapp.SortByCreateTime
	}

	return notebookapp.SortByCreateTime
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

	result, err := s.listNotebooksHandler.Handle(ctx,
		&notebookapp.ListNotebooksHandleQuery{
			OwnerId: "", // TODO get owner id from context
			Limit:   req.Limit,
			Offset:  req.Offset,
			SortBy:  req.SortBy.ToSortBy(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	notebooks := make([]*ListNotebookItemResponse, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		notebooks = append(notebooks, &ListNotebookItemResponse{
			Id:          notebook.Id.String(),
			Name:        notebook.Name,
			Desc:        notebook.Description,
			SourceCount: notebook.SourceCount,
			UpdatedAt:   notebook.UpdateTime.Value(),
			CreatedAt:   notebook.CreateTime.Value(),
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
	Sources []*schema.Source `json:"sources"`
	Limit   int              `json:"limit"`
	Offset  int              `json:"offset"`
	HasMore bool             `json:"has_more"`
}

func (s *Server) ListNotebookSources(ctx context.Context, c *app.RequestContext) {
	var req ListNotebookSourcesRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.notebookLogic.ListNotebookSources(ctx,
		&notebook.ListNotebookSourcesParams{
			NotebookId: req.Id,
			Limit:      req.Limit,
			Offset:     req.Offset,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ListNotebookSourcesResponse{
		Sources: schema.ToSources(result.Sources),
		Limit:   req.Limit,
		Offset:  req.Offset,
		HasMore: result.HasMore,
	})
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

	err = s.updateNotebookNameHandler.Handle(ctx, req.Id, req.Name)
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

type DeleteNotebookRequest struct {
	Id uuid.UUID `path:"id,required"`
}

func (s *Server) DeleteNotebook(ctx context.Context, c *app.RequestContext) {
	var req DeleteNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	err = s.deleteNotebookHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}

type ListNotebookStudioArtifactsRequest struct {
	Id     uuid.UUID `path:"id,required"`
	Limit  int       `query:"limit"      validate:"omitempty,min=1,max=50"`
	Offset int       `query:"offset"     validate:"min=0"`
}

const (
	defaultNotebookStudioArtifactsLimit = 50
)

func (r *ListNotebookStudioArtifactsRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebookStudioArtifactsLimit
	}
	return nil
}

type NotebookStudioArtifactResponse struct {
	Artifacts []*schema.ArtifactResult `json:"artifacts"`
	Limit     int                      `json:"limit"`
	Offset    int                      `json:"offset"`
	HasMore   bool                     `json:"has_more"`
}

func (s *Server) ListNotebookStudioArtifacts(ctx context.Context, c *app.RequestContext) {
	var req ListNotebookStudioArtifactsRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.studioLogic.ListNotebookArtifacts(ctx,
		&studio.ListNotebookArtifactsParams{
			NotebookId: req.Id,
			Limit:      req.Limit,
			Offset:     req.Offset,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, NotebookStudioArtifactResponse{
		Artifacts: schema.ToArtifactResults(result.Artifacts),
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   result.HasMore,
	})
}
