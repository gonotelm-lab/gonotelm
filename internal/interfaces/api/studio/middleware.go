package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/gonotelm-lab/gonotelm/internal/application/artifact"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Deps struct {
	GenerateHandler *artifact.GenerateArtifactHandler
	StatusHandler   *artifact.GetArtifactStatusHandler
	ListHandler     *artifact.ListArtifactsHandler
	RetryHandler    *artifact.RetryArtifactHandler
	CancelHandler   *artifact.CancelArtifactHandler
	DeleteHandler   *artifact.DeleteArtifactHandler
}

func (d *Deps) CheckArtifactAccess(ctx context.Context, c *app.RequestContext) {
	taskId := c.Param("task_id")
	tid, err := uuid.ParseString(taskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id"))
		c.Abort()
		return
	}

	err = d.StatusHandler.CheckOwnership(ctx, artifactentity.NewArtifactIdFromUUID(tid))
	if err != nil {
		http.ErrResp(c, err)
		c.Abort()
		return
	}

	c.Next(ctx)
}
