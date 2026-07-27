package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteArtifactHandler struct {
	repo    artifactrepo.Repository
	flowc   flow.TaskClient
	storage StorageGateway
}

func NewDeleteArtifactHandler(repo artifactrepo.Repository, flowc flow.TaskClient, storage StorageGateway) *DeleteArtifactHandler {
	return &DeleteArtifactHandler{repo: repo, flowc: flowc, storage: storage}
}

func (h *DeleteArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.repo.FindById(ctx, cmd)
	if err != nil {
		return err
	}
	if !a.IsOwner(pkgcontext.GetUserId(ctx)) {
		return artifacterrors.ErrArtifactNotOwnedByUser
	}
	if !a.IsTerminal() && a.FlowTaskId != "" {
		if err := h.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
			return errors.WithMessage(err, "cancel flow task failed")
		}
	}
	if a.ResultKind.Storage() && a.Result != nil {
		storeKey := extractStoreKey(a.Result)
		if storeKey != "" {
			_ = h.storage.DeleteObject(ctx, storeKey)
		}
	}
	return h.repo.DeleteById(ctx, a.Id)
}
