package usecase

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteUseCase struct {
	repo    artifactrepo.Repository
	flowc   flow.TaskClient
	storage StorageGateway
}

func NewDelete(repo artifactrepo.Repository, flowc flow.TaskClient, storage StorageGateway) *DeleteUseCase {
	return &DeleteUseCase{repo: repo, flowc: flowc, storage: storage}
}

func (u *DeleteUseCase) Execute(ctx context.Context, artifactId valobj.Id) error {
	a, err := u.repo.FindById(ctx, artifactId)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if !a.IsTerminal() && a.FlowTaskId != "" {
		if err := u.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
			return errors.WithMessage(err, "cancel flow task failed")
		}
	}
	if a.ResultKind.Storage() && a.Result != nil {
		storeKey := extractStoreKey(a.Result)
		if storeKey != "" {
			_ = u.storage.DeleteObject(ctx, storeKey)
		}
	}
	return u.repo.DeleteById(ctx, a.Id)
}
