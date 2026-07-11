package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	usecase "github.com/gonotelm-lab/gonotelm/internal/application/artifact/usecase"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

func (d *Deps) Generate(ctx context.Context, c *app.RequestContext) {
	var req GenerateRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := d.GenerateUC.Execute(ctx, &usecase.GenerateRequest{
		NotebookId:  req.NotebookId,
		Kind:        req.Kind,
		SourceIds:   req.SourceIds,
		InfoGraphic: req.InfoGraphic.toPayload(),
		AudioOverview: req.AudioOverview.toPayload(),
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, GenerateResponse{TaskId: resp.ArtifactId.String()})
}
