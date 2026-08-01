package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteChatContextHandler struct {
	*baseHandler
	chatContextMessageRepo chatrepo.ContextMessageRepository
}

func NewDeleteChatContextHandler(chatRepo chatrepo.ChatRepository, chatContextMessageRepo chatrepo.ContextMessageRepository) *DeleteChatContextHandler {
	return &DeleteChatContextHandler{
		baseHandler:            newBaseHandler(chatRepo),
		chatContextMessageRepo: chatContextMessageRepo,
	}
}

type DeleteChatContextCommand struct {
	ChatId valobj.Id
}

func (h *DeleteChatContextHandler) Handle(ctx context.Context, cmd *DeleteChatContextCommand) error {
	if _, err := h.commonHandle(ctx, cmd.ChatId); err != nil {
		return err
	}

	if err := h.chatContextMessageRepo.Destroy(ctx, cmd.ChatId); err != nil {
		return errors.WithMessage(err, "clear chat context failed")
	}

	return nil
}
