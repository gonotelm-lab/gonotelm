package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	artifactentity "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/entity"
	artifactrepo "github.com/gonotelm-lab/gonotelm/internal/domain/artifact/repository"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/flow"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const notebookArtifactsPageSize = 100

// OnNotebookEventHandler 停掉在途生成任务并删除产物自己的对象和数据记录
type OnNotebookEventHandler struct {
	artifactRepo   artifactrepo.Repository
	flowc          flow.TaskClient
	cleanupSweeper *cleanup.Sweeper
}

func NewOnNotebookEventHandler(
	artifactRepo artifactrepo.Repository,
	flowc flow.TaskClient,
	cleanupSweeper *cleanup.Sweeper,
) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{artifactRepo: artifactRepo, flowc: flowc, cleanupSweeper: cleanupSweeper}
}

func (h *OnNotebookEventHandler) Handle(
	ctx context.Context,
	evt *notebookevent.Event,
) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	artifacts, err := h.listNotebookArtifacts(ctx, notebookId)
	if err != nil {
		return errors.WithStack(err)
	}

	cancelled := h.cancelInFlight(ctx, artifacts)

	if err := h.cleanupSweeper.SweepDeletedNotebookArtifacts(ctx, notebookId, artifacts); err != nil {
		slog.ErrorContext(ctx, "failed to clean up artifacts for deleted notebook",
			slog.String("notebook_id", notebookId.String()),
			slog.Any("err", err),
		)
	}

	slog.InfoContext(ctx, "cleaned up artifacts for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("artifact_count", len(artifacts)),
		slog.Int("cancelled_count", cancelled),
	)

	return nil
}

// Cancel 是协作式的：取消失败只记日志，晚到的对象由 syncer 兜底。
func (h *OnNotebookEventHandler) cancelInFlight(
	ctx context.Context,
	artifacts []*artifactentity.Artifact,
) int {
	cancelled := 0

	for _, artifact := range artifacts {
		if artifact.IsTerminal() || artifact.FlowTaskId == "" {
			continue
		}

		if err := h.flowc.Cancel(ctx, artifact.FlowTaskId); err != nil {
			slog.WarnContext(ctx, "cancel in-flight artifact task failed",
				slog.String("notebook_id", artifact.NotebookId.String()),
				slog.String("artifact_id", artifact.Id.String()),
				slog.String("flow_task_id", artifact.FlowTaskId),
				slog.Any("err", err),
			)

			continue
		}

		cancelled++
	}

	return cancelled
}

func (h *OnNotebookEventHandler) listNotebookArtifacts(
	ctx context.Context,
	notebookId valobj.Id,
) ([]*artifactentity.Artifact, error) {
	artifacts := make([]*artifactentity.Artifact, 0, notebookArtifactsPageSize)

	for offset := 0; ; offset += notebookArtifactsPageSize {
		page, err := h.artifactRepo.ListByNotebookId(ctx, notebookId, &artifactrepo.ListSpec{
			Limit:  notebookArtifactsPageSize,
			Offset: offset,
		})
		if err != nil {
			return nil, errors.WithMessagef(err, "list artifacts failed, notebook_id=%s", notebookId)
		}
		if len(page) == 0 {
			break
		}

		artifacts = append(artifacts, page...)

		if len(page) < notebookArtifactsPageSize {
			break
		}
	}

	return artifacts, nil
}

func RegisterNotebookEventConsumer(
	bus eventbus.InProcessEventBus,
	handler *OnNotebookEventHandler,
) error {
	return bus.Subscribe(notebookevent.TopicNotebookEvent,
		func(ctx context.Context, evt event.Event) error {
			nbEvt, err := eventbus.AssertEvent[*notebookevent.Event](evt)
			if err != nil {
				return err
			}
			return handler.Handle(ctx, nbEvt)
		},
	)
}
