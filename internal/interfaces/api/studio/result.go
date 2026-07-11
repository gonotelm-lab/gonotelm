package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	usecase "github.com/gonotelm-lab/gonotelm/internal/application/artifact/usecase"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (d *Deps) GetResult(ctx context.Context, c *app.RequestContext) {
	taskId := c.Param("task_id")
	tid, err := uuid.ParseString(taskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id"))
		return
	}

	resp, err := d.StatusUC.Execute(ctx, &usecase.StatusRequest{ArtifactId: tid})
	if err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, StatusResponse{
		TaskId:      taskId,
		Status:      resp.Status,
		Title:       resp.Title,
		Content:     string(resp.Result),
		ContentKind: resp.ResultKind,
		FlowError:   resp.FlowError,
	})
}
