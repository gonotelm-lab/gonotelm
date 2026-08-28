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
	suggestionRepo         chatrepo.SuggestionRepository
}

func NewDeleteChatContextHandler(
	chatRepo chatrepo.ChatRepository,
	chatContextMessageRepo chatrepo.ContextMessageRepository,
	suggestionRepo chatrepo.SuggestionRepository,
) *DeleteChatContextHandler {
	return &DeleteChatContextHandler{
		baseHandler:            newBaseHandler(chatRepo),
		chatContextMessageRepo: chatContextMessageRepo,
		suggestionRepo:         suggestionRepo,
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

	if err := h.suggestionRepo.Delete(ctx, cmd.ChatId); err != nil {
		return errors.WithMessage(err, "clear chat suggestions failed")
	}

	return nil
}
