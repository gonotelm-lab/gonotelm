package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type GetRunningTaskHandler struct {
	*baseHandler
	streamTaskRepo chatrepo.StreamTaskRepository
}

func NewGetRunningTaskHandler(
	chatRepo chatrepo.ChatRepository,
	streamTaskRepo chatrepo.StreamTaskRepository,
) *GetRunningTaskHandler {
	return &GetRunningTaskHandler{
		baseHandler:    newBaseHandler(chatRepo),
		streamTaskRepo: streamTaskRepo,
	}
}

type GetRunningTaskQuery struct {
	ChatId valobj.Id
}

func (h *GetRunningTaskHandler) Handle(
	ctx context.Context,
	query *GetRunningTaskQuery,
) (valobj.Id, error) {
	if _, err := h.commonHandle(ctx, query.ChatId); err != nil {
		return valobj.Id{}, err
	}

	userId := pkgcontext.GetUserId(ctx)
	task, err := h.streamTaskRepo.FindByUserAndChat(ctx, userId, query.ChatId)
	if err != nil {
		if errors.Is(err, chaterrors.ErrStreamTaskNotFound) {
			return valobj.Id{}, chaterrors.ErrStreamTaskNotFound
		}
		return valobj.Id{}, errors.WithMessagef(err, "find running stream task failed, chat_id=%s", query.ChatId)
	}

	if task == nil || !task.Status.IsRunning() {
		return valobj.Id{}, chaterrors.ErrStreamTaskNotFound
	}

	return task.Id, nil
}
