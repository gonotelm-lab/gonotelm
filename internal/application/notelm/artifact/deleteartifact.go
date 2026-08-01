package artifact

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteArtifactHandler struct {
	*baseHandler
	flowc   flow.TaskClient
	storage adapter.StorageGateway
}

func NewDeleteArtifactHandler(repo artifactrepo.Repository, flowc flow.TaskClient, storage adapter.StorageGateway) *DeleteArtifactHandler {
	return &DeleteArtifactHandler{baseHandler: newBaseHandler(repo), flowc: flowc, storage: storage}
}

func (h *DeleteArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.handle(ctx, cmd)
	if err != nil {
		return err
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
