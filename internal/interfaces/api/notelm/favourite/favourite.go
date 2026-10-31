package favourite

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (s *Server) CreateFavourite(ctx context.Context, c *app.RequestContext) {
	var req schema.CreateFavouriteRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	view, err := s.createHandler.Handle(ctx, req.Name)
	if err != nil {
		http.ErrResp(c, err)
		return
	}
	http.OkResp(c, &schema.CreateFavouriteResponse{Id: view.Id.String()})
}

func (s *Server) ListFavourites(ctx context.Context, c *app.RequestContext) {
	var req schema.ListFavouritesRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.listHandler.Handle(ctx, req.Limit, req.Offset)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	items := make([]*schema.FavouriteResponse, 0, len(result.Favourites))
	for _, f := range result.Favourites {
		items = append(items, schema.NewFavouriteResponse(f))
	}
	http.OkResp(c, &schema.ListFavouritesResponse{
		Favourites: items,
		Limit:      result.Limit,
		Offset:     result.Offset,
		HasMore:    result.HasMore,
	})
}

func (s *Server) RenameFavourite(ctx context.Context, c *app.RequestContext) {
	var req schema.RenameFavouriteRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	view, err := s.renameHandler.Handle(ctx, valobj.Id(req.Id), req.Name)
	if err != nil {
		http.ErrResp(c, err)
		return
	}
	http.OkResp(c, schema.NewFavouriteResponse(view))
}

func (s *Server) DeleteFavourite(ctx context.Context, c *app.RequestContext) {
	var req schema.DeleteFavouriteRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	if err := s.deleteHandler.Handle(ctx, valobj.Id(req.Id)); err != nil {
		http.ErrResp(c, err)
		return
	}
	http.OkResp(c, nil)
}
