package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	artifactapp "github.com/gonotelm-lab/gonotelm/internal/application/notelm/artifact"
	"github.com/gonotelm-lab/gonotelm/internal/interfaces/api/notelm/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (s *Server) ConvertNoteToSource(ctx context.Context, c *app.RequestContext) {
	var req schema.ArtifactTaskIdRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := s.convertNoteToSourceHandler.Handle(ctx, &artifactapp.ConvertNoteToSourceCommand{
		ArtifactId: req.TaskId,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, schema.ConvertNoteToSourceResponse{
		SourceId: resp.SourceId.String(),
	})
}
