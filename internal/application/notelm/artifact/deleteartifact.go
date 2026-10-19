package artifact

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteArtifactHandler struct {
	*baseHandler
	flowc   flow.TaskClient
	storage adapter.ObjectStore
	cleaner *cleanup.Cleaner
}

func NewDeleteArtifactHandler(
	repo artifactrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	flowc flow.TaskClient,
	storage adapter.ObjectStore,
	cleaner *cleanup.Cleaner,
) *DeleteArtifactHandler {
	return &DeleteArtifactHandler{
		baseHandler: newBaseHandler(repo, notebookAccessService),
		flowc:       flowc,
		storage:     storage,
		cleaner:     cleaner,
	}
}

func (h *DeleteArtifactHandler) Handle(ctx context.Context, cmd valobj.Id) error {
	a, err := h.checkManageAccess(ctx, cmd)
	if err != nil {
		return err
	}
	if !a.IsTerminal() && a.FlowTaskId != "" {
		if err := h.flowc.Cancel(ctx, a.FlowTaskId); err != nil {
			return errors.WithMessage(err, "cancel flow task failed")
		}
	}

	var storeKey valobj.StoreKey
	if a.ResultKind.Storage() && len(a.Result) > 0 {
		storeKey = extractStoreKey(a.Result)
	}

	// 先注册兜底再删对象
	if err := h.cleaner.ScheduleArtifactCleanup(
		ctx,
		a.NotebookId,
		a.Id,
		[]valobj.StoreKey{storeKey},
	); err != nil {
		slog.ErrorContext(ctx, "register artifact cleanup checkpoint failed",
			slog.String("artifact_id", a.Id.String()),
			slog.String("notebook_id", a.NotebookId.String()),
			slog.Any("err", err),
		)
	}

	if storeKey.Valid() {
		_ = h.storage.DeleteObject(ctx, storeKey)
	}

	return h.repo.DeleteById(ctx, a.Id)
}
