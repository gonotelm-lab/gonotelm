package chat

import (
	"context"
	"log/slog"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookerrors "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/errors"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/eventbus"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type baseHandler struct {
	chatRepo              chatrepo.ChatRepository
	notebookAccessService *notebookservice.AccessService
}

func newBaseHandler(
	chatRepo chatrepo.ChatRepository,
	notebookAccessService *notebookservice.AccessService,
) *baseHandler {
	return &baseHandler{
		chatRepo:              chatRepo,
		notebookAccessService: notebookAccessService,
	}
}

// commonHandle 校验：
// 1. chat 存在；
// 2. 当前用户对所属 notebook 有 view 权限；
// 3. chat 属于当前用户（chat 每人私有）。
func (h *baseHandler) commonHandle(ctx context.Context, chatId valobj.Id) (*chatentity.Chat, error) {
	chat, err := h.chatRepo.FindById(ctx, chatId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get chat failed, chat_id=%s", chatId)
	}

	userId := pkgcontext.GetUserId(ctx)
	access, err := h.notebookAccessService.Resolve(ctx, chat.NotebookId, userId)
	if err != nil {
		return nil, errors.WithMessagef(err, "resolve notebook access failed, chat_id=%s", chatId)
	}

	if err := access.CanViewReturnErr(); err != nil {
		return nil, err
	}

	if chat.OwnerId.NotEqualsTo(userId) {
		return nil, notebookerrors.ErrNotebookNotFound // notebook的权限不足统一返回NOT_FOUND
	}

	return chat, nil
}

// ensureStreamAccess 校验流式任务属于当前用户的 chat，且当前用户仍有 notebook view 权限。
func (h *baseHandler) ensureStreamAccess(
	ctx context.Context,
	task *chatentity.StreamTask,
	chatId valobj.Id,
	userId valobj.Uid,
) error {
	if task.ChatId != chatId {
		return errors.ErrParams.Msgf("task chat_id mismatch, task_id=%s, chat_id=%s",
			task.Id, chatId)
	}

	if task.UserId.NotEqualsTo(userId) {
		return chaterrors.ErrStreamTaskNotFound
	}

	chat, err := h.chatRepo.FindById(ctx, chatId)
	if err != nil {
		return errors.WithMessagef(err, "get chat failed, chat_id=%s", chatId)
	}

	access, err := h.notebookAccessService.Resolve(ctx, chat.NotebookId, userId)
	if err != nil {
		return errors.WithMessagef(err, "resolve notebook access failed, chat_id=%s", chatId)
	}

	if err := access.CanViewReturnErr(); err != nil {
		return err
	}
	if chat.OwnerId.NotEqualsTo(userId) {
		return chaterrors.ErrStreamTaskNotFound
	}

	return nil
}

func publishStreamTaskDomainEvents(ctx context.Context, bus eventbus.Publisher, task *chatentity.StreamTask) {
	for _, evt := range task.PullEvents() {
		if err := bus.Publish(ctx, evt); err != nil {
			slog.ErrorContext(ctx, "publish stream task event failed",
				slog.Any("task_id", task.Id), slog.Any("err", err),
			)
		}
	}
}
