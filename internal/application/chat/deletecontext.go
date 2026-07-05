package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteChatContextHandler struct {
	chatRepo               chatrepo.Repository
	chatContextMessageRepo chatrepo.ContextMessageRepository
}

func NewDeleteChatContextHandler(
	chatRepo chatrepo.Repository,
	chatContextMessageRepo chatrepo.ContextMessageRepository,
) *DeleteChatContextHandler {
	return &DeleteChatContextHandler{
		chatRepo:               chatRepo,
		chatContextMessageRepo: chatContextMessageRepo,
	}
}

type DeleteChatContextCommand struct {
	ChatId valobj.Id
}

func (h *DeleteChatContextHandler) Handle(ctx context.Context, cmd *DeleteChatContextCommand) error {
	targetChat, err := h.chatRepo.FindById(ctx, cmd.ChatId)
	if err != nil {
		return errors.WithMessage(err, "find chat failed")
	}

	userId := pkgcontext.GetUserId(ctx)
	if targetChat.OwnerId != userId {
		return errors.ErrParams.Msgf("chat not belong to user, chat_id=%s", cmd.ChatId)
	}

	if err := h.chatContextMessageRepo.Destroy(ctx, cmd.ChatId); err != nil {
		return errors.WithMessage(err, "clear chat context failed")
	}

	return nil
}
