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

type OnNotebookEventHandler struct {
	pinRepo pinrepo.Repository
}

func NewOnNotebookEventHandler(pinRepo pinrepo.Repository) *OnNotebookEventHandler {
	return &OnNotebookEventHandler{pinRepo: pinRepo}
}

// 订阅notebook删除事件 notebook删除后需要去掉存在的pin记录
func (h *OnNotebookEventHandler) Handle(ctx context.Context, evt *notebookevent.Event) error {
	if evt.Action() != notebookevent.EventActionDelete {
		return nil
	}

	notebookId := evt.NotebookId()
	if err := h.pinRepo.DeleteByNotebookId(ctx, notebookId); err != nil {
		return errors.WithMessagef(err, "delete pins by notebook failed, notebook_id=%s", notebookId)
	}

	slog.InfoContext(ctx, "cleaned up pins for deleted notebook",
		slog.String("notebook_id", notebookId.String()),
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
