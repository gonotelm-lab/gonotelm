package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type OnNotebookEventHandler struct {
	chatRepo               chatrepo.ChatRepository
	chatMessageRepo        chatrepo.MessageRepository
	chatContextMessageRepo chatrepo.ContextMessageRepository
}

func NewOnNotebookEventHandler(
	chatRepo chatrepo.ChatRepository,
	chatMessageRepo chatrepo.MessageRepository,
	chatContextMessageRepo chatrepo.ContextMessageRepository,
) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{
		chatRepo:               chatRepo,
		chatMessageRepo:        chatMessageRepo,
		chatContextMessageRepo: chatContextMessageRepo,
	}
}

func (h *OnNotebookEventHandler) Handle(
	ctx context.Context,
	evt *notebookevent.Event,
) error {
	if evt.Action() != notebookevent.EventActionDelete {
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
		slog.ErrorContext(ctx, "failed to delete chat messages",
			slog.Any("err", err), slog.String("notebook_id", notebookId.String()),
		)
	}

	if err := h.chatContextMessageRepo.BatchDestroy(ctx, chatIds); err != nil {
		slog.ErrorContext(ctx, "failed to delete chat context",
			slog.Any("err", err), slog.String("notebook_id", notebookId.String()),
		)
	}

	if err := h.chatRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		slog.ErrorContext(ctx, "failed to delete chats",
			slog.Any("err", err), slog.String("notebook_id", notebookId.String()),
		)
	}

	slog.InfoContext(ctx, "cleaned up chats for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
		slog.Int("chat_count", len(chatIds)),
	)

	return nil
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
