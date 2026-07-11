package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type CancelUseCase struct {
	repo  artifactrepo.Repository
	flowc flow.TaskClient
}

func NewCancel(repo artifactrepo.Repository, flowc flow.TaskClient) *CancelUseCase {
	return &CancelUseCase{repo: repo, flowc: flowc}
}

func (u *CancelUseCase) Execute(ctx context.Context, artifactId valobj.Id) error {
	a, err := u.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if a.IsTerminal() {
		return artifacterrors.ErrCannotCancelInState
	}
	if a.FlowTaskId == "" {
		return artifacterrors.ErrInvalidFlowTaskId
	}
	if err := u.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
		return err
	}
	return u.repo.UpdateStatus(ctx, a.Id, artifactentity.StatusCancelled, nil, "", "")
}
