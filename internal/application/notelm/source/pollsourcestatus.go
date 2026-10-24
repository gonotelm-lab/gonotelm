package source

import (
	"context"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gonotelm-lab/gonotelm/internal/core/adapter"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity"
	"github.com/gonotelm-lab/gonotelm/internal/domain/source/entity/vo"
	repo "github.com/gonotelm-lab/gonotelm/internal/domain/source/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type PollSourceStatusHandler struct {
	*baseHandler
	objectStore adapter.ObjectStore
	eventBus    eventbus.Publisher
}

func NewPollSourceStatusHandler(
	sourceRepo repo.Repository,
	notebookAccessService *notebookservice.AccessService,
	objectStore adapter.ObjectStore,
	eventBus eventbus.Publisher,
) *PollSourceStatusHandler {
	return &PollSourceStatusHandler{
		baseHandler: newBaseHandler(sourceRepo, notebookAccessService),
		objectStore: objectStore,
		eventBus:    eventBus,
	}
}

func (h *PollSourceStatusHandler) Handle(
	ctx context.Context,
	sourceId valobj.Id,
) (vo.SourceStatus, error) {
	return withSourceAccess(h.baseHandler, ctx, sourceId, accessView, h.pollStatus)
}

func (h *PollSourceStatusHandler) pollStatus(
	ctx context.Context,
	targetSource *entity.Source,
) (vo.SourceStatus, error) {
	if targetSource.Status.IsReady() {
		return targetSource.Status, nil
	}

	status := targetSource.Status
	if targetSource.Kind.IsFile() {
		var err error
		status, err = h.pollFileSourceStatus(ctx, targetSource)
		if err != nil {
			return "", errors.WithMessagef(err, "poll file source status failed, source_id=%s", targetSource.Id)
		}
	}

	return status, nil
}

func (h *PollSourceStatusHandler) pollFileSourceStatus(
	ctx context.Context,
	targetSource *entity.Source,
) (vo.SourceStatus, error) {
	if !targetSource.Status.IsUploading() {
		return targetSource.Status, nil
	}

	fileContent, err := targetSource.GetFileContent()
	if err != nil {
		return "", errors.WithMessagef(err, "get file content failed, source_id=%s", targetSource.Id)
	}

	// maybe is uploading, check if file already uploaded
	uploaded := true
	partial, objectInfo, err := h.objectStore.GetPartialObject(ctx, fileContent.StoreKey, 0, 3072)
	if err != nil {
		uploaded = false
		if !errors.Is(err, adapter.ErrObjectNotFound) {
			return "", errors.WithMessagef(err, "check file exist failed, store_key=%s", fileContent.StoreKey.String())
		}
	}

	if uploaded {
		detectedMime := mimetype.Detect(partial)
		// check if file is valid
		err = targetSource.CheckProcessable(&entity.CheckProcessableParams{
			TotalSize:   objectInfo.Size,
			ContentType: detectedMime,
		})

		if err != nil {
			targetSource.MarkInvalid() // without events
		} else {
			// uploaded, make it preparing
			targetSource.MarkPreparing() // with events
		}
		if err := h.updateSource(ctx, targetSource); err != nil {
			return "", errors.WithMessagef(err, "save source failed, source_id=%s", targetSource.Id)
		}

		// events handling
		for _, event := range targetSource.PullEvents() {
			if err = h.eventBus.Publish(ctx, event); err != nil {
				return "", errors.WithMessagef(err, "publish event failed, event=%+v, source_id=%s", event, targetSource.Id)
			}
		}
	}

	return targetSource.Status, nil
}
