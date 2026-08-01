package chat

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	chatRepo chatrepo.ChatRepository
}

func newBaseHandler(chatRepo chatrepo.ChatRepository) *baseHandler {
	return &baseHandler{
		chatRepo: chatRepo,
	}
}

// 所有handler都要先处理这个公共的操作
func (h *baseHandler) commonHandle(ctx context.Context, chatId valobj.Id) (*chatentity.Chat, error) {
	chat, err := h.chatRepo.FindById(ctx, chatId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get chat failed, chat_id=%s", chatId)
	}

	// check owner id
	userId := pkgcontext.GetUserId(ctx)
	if chat.OwnerId != userId {
		return nil, errors.WithMessagef(errors.ErrPermission, "chat access denied, chat_id=%s", chatId)
	}

	return chat, nil
}

func publishStreamTaskDomainEvents(ctx context.Context, bus eventbus.EventBus, task *chatentity.StreamTask) {
	for _, evt := range task.PullEvents() {
		if err := bus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish stream task event failed",
				slog.Any("task_id", task.Id), slog.Any("err", err),
			)
		}
	}
}
