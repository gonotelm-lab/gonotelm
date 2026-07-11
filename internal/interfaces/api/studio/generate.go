package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (d *Deps) Generate(ctx context.Context, c *app.RequestContext) {
	var req GenerateRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := d.GenerateHandler.Handle(ctx, &artifact.GenerateRequest{
		NotebookId:    req.NotebookId,
		Kind:          req.Kind,
		SourceIds:     req.SourceIds,
		InfoGraphic:   req.InfoGraphic.toPayload(),
		AudioOverview: req.AudioOverview.toPayload(),
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GenerateResponse{TaskId: resp.ArtifactId.String()})
}
