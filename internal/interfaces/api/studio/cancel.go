package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

func (d *Deps) Cancel(ctx context.Context, c *app.RequestContext) {
	taskId := c.Param("task_id")
	tid, err := uuid.ParseString(taskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id"))
		return
	}

	if err := d.CancelHandler.Handle(ctx, tid); err != nil {
		http.ErrResp(c, err)
		return
	}

	http.OkResp(c, nil)
}
