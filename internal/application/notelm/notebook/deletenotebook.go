package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteNotebookHandler struct {
	*baseHandler
	eventBus         eventbus.Publisher
	cleanupRegistrar *cleanup.NotebookCleanupRegistrar
}

func NewDeleteNotebookHandler(
	notebookRepo notebookrepo.Repository,
	eventBus eventbus.Publisher,
	cleanupRegistrar *cleanup.NotebookCleanupRegistrar,
) *DeleteNotebookHandler {
	return &DeleteNotebookHandler{
		baseHandler:      newBaseHandler(notebookRepo),
		eventBus:         eventBus,
		cleanupRegistrar: cleanupRegistrar,
	}
}

func (h *DeleteNotebookHandler) Handle(ctx context.Context, id valobj.Id) error {
	n, err := h.handle(ctx, id)
	if err != nil {
		if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
			return nil
		}
		return err
	}

	n.Delete()
	err = h.notebookRepo.Save(ctx, n)
	if err != nil {
		return errors.WithMessage(err, "delete notebook failed")
	}

	// 发布事件前注册：此时 source/artifact 记录还没删，拿得到对象 key
	if err := h.cleanupRegistrar.Register(ctx, id); err != nil {
		slog.ErrorContext(ctx, "register notebook cleanup checkpoint failed",
			slog.String("notebook_id", id.String()),
			slog.Any("err", err),
		)
	}

	for _, event := range n.PullEvents() {
		err = h.eventBus.Publish(ctx, event)
		if err != nil {
			slog.ErrorContext(ctx, "publish notebook deleted event failed", "error", err, "notebook_id", id)
		}
	}

	return nil
}
