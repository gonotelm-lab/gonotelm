package notebook

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/application/shared/cleanup"
	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type DeleteNotebookHandler struct {
	*baseHandler
	eventBus   eventbus.Publisher
	cleaner    *cleanup.Cleaner
	memberRepo notebookrepo.MemberRepository
}

func NewDeleteNotebookHandler(
	notebookRepo notebookrepo.Repository,
	notebookAccessService *notebookservice.AccessService,
	memberRepo notebookrepo.MemberRepository,
	eventBus eventbus.Publisher,
	cleaner *cleanup.Cleaner,
) *DeleteNotebookHandler {
	return &DeleteNotebookHandler{
		baseHandler: newBaseHandler(notebookRepo, notebookAccessService),
		eventBus:    eventBus,
		cleaner:     cleaner,
		memberRepo:  memberRepo,
	}
}

func (h *DeleteNotebookHandler) Handle(ctx context.Context, id valobj.Id) error {
	access, err := h.handleOwner(ctx, id)
	if err != nil {
		if errors.Is(err, notebookerrors.ErrNotebookNotFound) {
			return nil
		}
		return err
	}
	n := access.Notebook

	n.Delete()
	err = h.notebookRepo.Save(ctx, n)
	if err != nil {
		return errors.WithMessage(err, "delete notebook failed")
	}

	if err := h.memberRepo.DeleteByNotebookId(ctx, id); err != nil {
		slog.ErrorContext(ctx, "delete notebook members failed",
			slog.String("notebook_id", id.String()),
			slog.Any("err", err),
		)
	}

	// 发布事件前注册：此时 source/artifact 记录还没删，拿得到对象 key
	if err := h.cleaner.RegisterNotebookCleanup(ctx, id); err != nil {
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
