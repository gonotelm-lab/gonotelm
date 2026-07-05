package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookdomain "github.com/gonotelm-lab/gonotelm/internal/domain/notebook"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteNotebookChatsHandler struct {
	chatRepo               chatrepo.Repository
	chatMessageRepo        chatrepo.MessageRepository
	chatContextMessageRepo chatrepo.ContextMessageRepository
}

func NewDeleteNotebookChatsHandler(
	chatRepo chatrepo.Repository,
	chatMessageRepo chatrepo.MessageRepository,
	chatContextMessageRepo chatrepo.ContextMessageRepository,
) *DeleteNotebookChatsHandler {
	return &DeleteNotebookChatsHandler{
		chatRepo:               chatRepo,
		chatMessageRepo:        chatMessageRepo,
		chatContextMessageRepo: chatContextMessageRepo,
	}
}

func (h *DeleteNotebookChatsHandler) Handle(
	ctx context.Context,
	evt *notebookdomain.Event,
) error {
	if evt.Action() != notebookdomain.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	chats, err := h.chatRepo.ListByNotebookId(ctx, notebookId)
	if err != nil {
		return errors.WithMessagef(err, "list chats by notebook failed, notebook_id=%s", notebookId)
	}
	if len(chats) == 0 {
		return nil
	}

	chatIds := make([]valobj.Id, 0, len(chats))
	for _, chat := range chats {
		chatIds = append(chatIds, chat.Id)
	}

	if err := h.chatMessageRepo.DeleteByChatIds(ctx, chatIds); err != nil {
		return errors.WithMessagef(err, "delete chat messages failed, notebook_id=%s", notebookId)
	}

	if err := h.chatContextMessageRepo.BatchDestroy(ctx, chatIds); err != nil {
		return errors.WithMessagef(err, "delete chat context failed, notebook_id=%s", notebookId)
	}

	if err := h.chatRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		return errors.WithMessagef(err, "delete chats failed, notebook_id=%s", notebookId)
	}

	slog.InfoContext(ctx, "cleaned up chats for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("chat_count", len(chatIds)),
	)

	return nil
}

func RegisterNotebookDeletedConsumer(
	ctx context.Context,
	bus eventbus.EventBus,
	handler *DeleteNotebookChatsHandler,
) error {
	return eventbus.SubscribeNotebookDeleted(ctx, bus, handler.Handle)
}
