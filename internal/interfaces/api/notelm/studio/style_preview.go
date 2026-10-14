package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (s *Server) ListStylePreviews(ctx context.Context, c *app.RequestContext) {
	var req schema.ListStylePreviewsRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.listStylePreviewsHandler.Handle(ctx,
		&artifactapp.ListStylePreviewsQuery{
			Kind: artifactentity.Kind(req.Kind),
		})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	previews := make([]*schema.StylePreviewItem, 0, len(resp.Previews))
	for _, item := range resp.Previews {
		previews = append(previews, &schema.StylePreviewItem{
			VisualStyle: item.VisualStyle,
			PreviewUrl:  item.PreviewUrl,
		})
	}

	http.OkResp(c, schema.ListStylePreviewsResponse{
		Kind:               resp.Kind.String(),
		DefaultVisualStyle: resp.DefaultVisualStyle,
		Previews:           previews,
	})
}
