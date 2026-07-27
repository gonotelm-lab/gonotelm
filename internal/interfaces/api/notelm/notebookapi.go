package notelm

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/route"
	chatapp "github.com/gonotelm-lab/gonotelm/internal/application/chat"
	notebookapp "github.com/gonotelm-lab/gonotelm/internal/application/notebook"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/source"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	pkgctx "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (s *Server) registerNotebooksRoutes(g *route.RouterGroup) {
	// POST /api/v1/notebooks
	g.POST("/notebooks", s.CreateNotebook)
	// GET /api/v1/notebooks
	g.GET("/notebooks", s.ListNotebooks)

	notebookIdGroup := g.Group("/notebooks/:id")
	notebookIdGroup.Use(s.checkNotebookUserId)
	{
		// GET /api/v1/notebooks/:id
		notebookIdGroup.GET("", s.GetNotebook)
		// DELETE /api/v1/notebooks/:id
		notebookIdGroup.DELETE("", s.DeleteNotebook)
		// PATCH /api/v1/notebooks/:id
		notebookIdGroup.PATCH("", s.UpdateNotebook)
		// GET /api/v1/notebooks/:id/sources
		notebookIdGroup.GET("/sources", s.ListNotebookSources)
		// GET /api/v1/notebooks/:id/artifacts
		notebookIdGroup.GET("/artifacts", s.ListNotebookStudioArtifacts)
		// POST /api/v1/notebooks/:id/artifacts
		notebookIdGroup.POST("/artifacts", s.GenerateStudioArtifact)
		// POST /api/v1/notebooks/:id/sources
		notebookIdGroup.POST("/sources", s.CreateSource)
		// POST /api/v1/notebooks/:id/chats
		notebookIdGroup.POST("/chats", s.GetOrCreateNotebookChat)
	}
}

func (s *Server) checkNotebookUserId(ctx context.Context, c *app.RequestContext) {
	notebookId := c.Param("id")
	nid, err := uuid.ParseString(notebookId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid notebook_id: %s", notebookId))
		return
	}

	err = s.checkNotebookAccessHandler.Handle(ctx, nid)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	c.Next(ctx)
}

// Create new notebook
func (s *Server) CreateNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.CreateNotebookRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	userId := pkgctx.GetUserId(ctx)
	id, err := s.createNotebookHandler.Handle(ctx,
		&notebookapp.CreateNotebookHandleCommand{
			Name:    req.Name,
			Desc:    req.Desc,
			OwnerId: userId,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.CreateNotebookResponse{Id: id.String()})
}

func (s *Server) GetNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.GetNotebookRequest
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

	http.OkResp(c, schema.GetNotebookResponse{
		Id:          notebook.Id.String(),
		Name:        notebook.Name,
		Desc:        notebook.Description,
		SourceCount: notebook.SourceCount,
		UpdatedAt:   notebook.UpdateTime.Value(),
		CreatedAt:   notebook.CreateTime.Value(),
	})
}

func (s *Server) ListNotebooks(ctx context.Context, c *app.RequestContext) {
	var req schema.ListNotebooksRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	userId := pkgctx.GetUserId(ctx)
	result, err := s.listNotebooksHandler.Handle(ctx,
		&notebookapp.ListNotebooksHandleQuery{
			OwnerId: userId,
			Limit:   req.Limit,
			Offset:  req.Offset,
			SortBy:  req.SortBy.ToSortBy(),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	notebooks := make([]*schema.ListNotebookItemResponse, 0, len(result.Notebooks))
	for _, notebook := range result.Notebooks {
		notebooks = append(notebooks, &schema.ListNotebookItemResponse{
			Id:          notebook.Id.String(),
			Name:        notebook.Name,
			Desc:        notebook.Description,
			SourceCount: notebook.SourceCount,
			UpdatedAt:   notebook.UpdateTime.Value(),
			CreatedAt:   notebook.CreateTime.Value(),
		})
	}

	http.OkResp(c, schema.ListNotebooksResponse{
		Notebooks: notebooks,
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   result.HasMore,
	})
}

func (s *Server) ListNotebookSources(ctx context.Context, c *app.RequestContext) {
	var req schema.ListNotebookSourcesRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.listSourcesHandler.Handle(ctx,
		&sourceapp.ListSourcesQuery{
			NotebookId: req.Id,
			Limit:      req.Limit,
			Offset:     req.Offset,
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ListNotebookSourcesResponse{
		Sources: schema.ToSourcesFromDomainDetails(result.Sources),
		Limit:   req.Limit,
		Offset:  req.Offset,
		HasMore: result.HasMore,
	})
}

func (s *Server) UpdateNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.UpdateNotebookRequest
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

func (s *Server) GetOrCreateNotebookChat(ctx context.Context, c *app.RequestContext) {
	var req schema.GetNotebookChatRequest
	err := c.BindAndValidate(&req)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	chat, err := s.createChatHandler.Handle(ctx,
		&chatapp.CreateChatCommand{
			NotebookId: req.Id,
			OwnerId:    pkgctx.GetUserId(ctx),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.GetNotebookChatResponse{
		ChatId: chat.Id.String(),
	})
}

func (s *Server) DeleteNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteNotebookRequest
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
