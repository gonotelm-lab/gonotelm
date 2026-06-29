package source

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	sourceentity "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	sourcevo "github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	sourcerepo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type PollSourceStatusHandler struct {
	sourceRepo  sourcerepo.Repository
	storageRepo sourcerepo.StorageRepository
	eventBus    eventbus.EventBus
}

func NewPollSourceStatusHandler(
	sourceRepo sourcerepo.Repository,
	storageRepo sourcerepo.StorageRepository,
	eventBus eventbus.EventBus,
) *PollSourceStatusHandler {
	return &PollSourceStatusHandler{
		sourceRepo:  sourceRepo,
		storageRepo: storageRepo,
		eventBus:    eventBus,
	}
}

func (h *PollSourceStatusHandler) Handle(
	ctx context.Context,
	sourceId valobj.Id,
) (sourcevo.SourceStatus, error) {
	targetSource, err := h.sourceRepo.FindById(ctx, sourceId)
	if err != nil {
		return "", errors.WithMessagef(err, "find source failed, source_id=%s", sourceId)
	}

	if targetSource.Status.IsReady() {
		return targetSource.Status, nil
	}

	status := targetSource.Status
	if targetSource.Kind.IsFile() {
		// trigger file source preparation
		status, err = h.pollFileSourceStatus(ctx, targetSource)
		if err != nil {
			return "", errors.WithMessagef(err, "poll file source status failed, source_id=%s", sourceId)
		}
	}

	return status, nil
}

func (h *PollSourceStatusHandler) pollFileSourceStatus(
	ctx context.Context,
	targetSource *sourceentity.Source,
) (sourcevo.SourceStatus, error) {
	if !targetSource.Status.IsUploading() {
		return targetSource.Status, nil
	}

	fileContent, err := targetSource.GetFileContent()
	if err != nil {
		return "", errors.WithMessagef(err, "get file content failed, source_id=%s", targetSource.Id)
	}

	// maybe is uploading, check if file already uploaded
	uploaded, err := h.storageRepo.CheckExist(ctx, fileContent.StoreKey)
	if err != nil {
		return "", errors.WithMessagef(err, "check file exist failed, store_key=%s", fileContent.StoreKey)
	}

	if uploaded {
		// uploaded, make it preparing
		targetSource.MarkPreparing()
		// events handling
		err = h.sourceRepo.Save(ctx, targetSource)
		if err != nil {
			return "", errors.WithMessagef(err, "save source failed, source_id=%s", targetSource.Id)
		}

		for _, event := range targetSource.PullEvents() {
			err = h.eventBus.Publish(ctx, event)
			if err != nil {
				return "", errors.WithMessagef(err, "publish event failed, event=%+v", event)
			}
		}
	}

	return targetSource.Status, nil
}
