package eventhandle

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/event"
	notebookevent "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/event"
	pinrepo "github.com/gonotelm-lab/gonotelm/internal/domain/pin/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type OnNotebookMemberEventHandler struct {
	pinRepo pinrepo.Repository
}

func NewOnNotebookMemberEventHandler(pinRepo pinrepo.Repository) *OnNotebookMemberEventHandler {
	return &OnNotebookMemberEventHandler{pinRepo: pinRepo}
}

func (h *OnNotebookMemberEventHandler) Handle(ctx context.Context, evt *notebookevent.MemberEvent) error {
	if evt.Action() != notebookevent.EventActionMemberDeleted {
		return nil
	}

	notebookId := evt.NotebookId()
	userId := evt.UserId()
	if err := h.pinRepo.Delete(ctx, userId, notebookId); err != nil {
		return errors.WithMessagef(err,
			"delete pin for removed member failed, notebook_id=%s user_id=%s",
			notebookId, userId,
		)
	}

	slog.InfoContext(ctx, "cleaned up pin for removed notebook member",
		slog.String("notebook_id", notebookId.String()),
		slog.String("user_id", userId.String()),
	)
	return nil
}

func RegisterNotebookMemberEventConsumer(
	bus eventbus.InProcessEventBus,
	handler *OnNotebookMemberEventHandler,
) error {
	return bus.Subscribe(notebookevent.TopicNotebookMemberEvent,
		func(ctx context.Context, evt event.Event) error {
			memberEvt, err := eventbus.AssertEvent[*notebookevent.MemberEvent](evt)
			if err != nil {
				return err
			}
			return handler.Handle(ctx, memberEvt)
		},
	)
}
