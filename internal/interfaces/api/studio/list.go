package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	usecase "github.com/gonotelm-lab/gonotelm/internal/application/artifact/usecase"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
)

const (
	defaultNotebookArtifactsLimit = 50
)

func (r *ListNotebookArtifactsRequest) Validate() error {
	if r.Limit == 0 {
		r.Limit = defaultNotebookArtifactsLimit
	}
	return nil
}

func (d *Deps) ListNotebookArtifacts(ctx context.Context, c *app.RequestContext) {
	var req ListNotebookArtifactsRequest
	if err := c.BindAndValidate(&req); err != nil {
		http.ErrResp(c, err)
		return
	}

	resp, err := d.ListUC.Execute(ctx, &usecase.ListRequest{
		NotebookId: req.Id,
		Limit:      req.Limit,
		Offset:     req.Offset,
	})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, ListNotebookArtifactsResponse{
		Artifacts: ToArtifactResults(resp.Artifacts),
		Limit:     req.Limit,
		Offset:    req.Offset,
		HasMore:   resp.HasMore,
	})
}
