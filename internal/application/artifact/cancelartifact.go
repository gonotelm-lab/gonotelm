package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifacterrors "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/errors"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
)

type CancelArtifactHandler struct {
	repo  artifactrepo.Repository
	flowc flow.TaskClient
}

func NewCancelArtifactHandler(repo artifactrepo.Repository, flowc flow.TaskClient) *CancelArtifactHandler {
	return &CancelArtifactHandler{repo: repo, flowc: flowc}
}

func (h *CancelArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.repo.FindById(ctx, cmd)
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
	if err := h.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
		return err
	}
	return h.repo.UpdateStatus(ctx, a.Id, artifactentity.StatusCancelled, nil, "", "")
}
