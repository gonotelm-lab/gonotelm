package notebook

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

// 置顶notebook
func (s *Server) PinNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.PinNotebookRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.pinNotebookHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.PinNotebookResponse{
		IsPinned: result.IsPinned,
		PinnedAt: result.PinnedAt,
	})
}

// 取消notebook置顶
func (s *Server) UnpinNotebook(ctx context.Context, c *app.RequestContext) {
	var req schema.PinNotebookRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	result, err := s.unpinNotebookHandler.Handle(ctx, req.Id)
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.PinNotebookResponse{
		IsPinned: result.IsPinned,
		PinnedAt: result.PinnedAt,
	})
}
