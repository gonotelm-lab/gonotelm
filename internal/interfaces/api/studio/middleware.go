package studio

import (
	"context"

	"github.com/cloudwego/hertz/pkg/app"
	usecase "github.com/gonotelm-lab/gonotelm/internal/application/artifact/usecase"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/http"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"
)

type Deps struct {
	GenerateUC *usecase.GenerateUseCase
	StatusUC   *usecase.StatusUseCase
	ListUC     *usecase.ListUseCase
	RetryUC    *usecase.RetryUseCase
	CancelUC   *usecase.CancelUseCase
	DeleteUC   *usecase.DeleteUseCase
}

func (d *Deps) CheckArtifactAccess(ctx context.Context, c *app.RequestContext) {
	taskId := c.Param("task_id")
	tid, err := uuid.ParseString(taskId)
	if err != nil {
		http.ErrResp(c, errors.ErrParams.Msgf("invalid task_id"))
		c.Abort()
		return
	}

	err = d.StatusUC.CheckOwnership(ctx, artifactentity.NewArtifactIdFromUUID(tid))
	if err != nil {
		http.ErrResp(c, err)
		c.Abort()
		return
	}

	c.Next(ctx)
}
