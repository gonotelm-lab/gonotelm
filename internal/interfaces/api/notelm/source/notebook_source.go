package source

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	sourceapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/source"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// ListNotebookSources 笔记本详情页的来源列表，挂在 /notebooks/:id/sources 上
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
