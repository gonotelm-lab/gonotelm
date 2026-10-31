package favourite

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (s *Server) ListFavouriteNotebooks(ctx context.Context, c *app.RequestContext) {
	var req schema.ListFavouriteNotebooksRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.listNotebookHandler.Handle(ctx, valobj.Id(req.Id), req.Limit, req.Offset)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	items := make([]*schema.FavouriteNotebookItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, schema.NewFavouriteNotebookItemResponse(item))
	}
	http.OkResp(c, &schema.ListFavouriteNotebooksResponse{
		Items:   items,
		Limit:   result.Limit,
		Offset:  result.Offset,
		HasMore: result.HasMore,
	})
}

func (s *Server) AddFavouriteNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.AddFavouriteNotebookRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.addNotebookHandler.Handle(ctx, valobj.Id(req.Id), valobj.Id(req.NotebookId)); err != nil {
		http.ErrResp(c, err)
		return
	}
	http.OkResp(c, nil)
}

func (s *Server) RemoveFavouriteNotebooks(ctx context.Context, c *app.RequestContext) {
	var req schema.RemoveFavouriteNotebooksRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	ids := make([]valobj.Id, 0, len(req.NotebookIds))
	for _, id := range req.NotebookIds {
		ids = append(ids, valobj.Id(id))
	}
	if err := s.removeNotebookHandler.Handle(ctx, valobj.Id(req.Id), ids); err != nil {
		http.ErrResp(c, err)
		return
	}
	http.OkResp(c, nil)
}
