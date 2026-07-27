package chat

import (
	"context"
	"time"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

const defaultStreamTaskTTL = 100 * time.Minute

type AbortStreamHandler struct {
	streamTaskRepo chatrepo.StreamTaskRepository
}

func NewAbortStreamHandler(streamTaskRepo chatrepo.StreamTaskRepository) *AbortStreamHandler {
	return &AbortStreamHandler{
		streamTaskRepo: streamTaskRepo,
	}
}

type AbortStreamCommand struct {
	ChatId valobj.Id
	TaskId valobj.Id
}

func (h *AbortStreamHandler) Handle(ctx context.Context, cmd *AbortStreamCommand) error {
	task, err := h.streamTaskRepo.FindById(ctx, cmd.TaskId)
	if err != nil {
		if errors.Is(err, chaterrors.ErrStreamTaskNotFound) {
			return errors.ErrParams.Msgf("task not found, task_id=%s", cmd.TaskId)
		}

		return errors.WithMessage(err, "find stream task failed")
	}

	userId := pkgcontext.GetUserId(ctx)
	if err := canAccessStreamTask(task, cmd.ChatId, userId); err != nil {
		return err
	}

	if !task.Status.IsRunning() {
		return nil
	}

	task.Status = chatentity.StreamTaskStatusAborted
	if err := h.streamTaskRepo.Save(ctx, task); err != nil {
		return errors.WithMessage(err, "save aborted stream task failed")
	}

	if err := h.streamTaskRepo.SetStreamTTL(ctx, cmd.TaskId, defaultStreamTaskTTL); err != nil {
		return errors.WithMessage(err, "set stream task ttl failed")
	}

	return nil
}
