package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookservice "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/service"
	pkgcontext "github.com/gonotelm-lab/gonotelm/pkg/context"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CreateChatHandler struct {
	notebookAccessService *notebookservice.AccessService
	chatRepo              chatrepo.ChatRepository
}

func NewCreateChatHandler(
	notebookAccessService *notebookservice.AccessService,
	chatRepo chatrepo.ChatRepository,
) *CreateChatHandler {
	return &CreateChatHandler{
		notebookAccessService: notebookAccessService,
		chatRepo:              chatRepo,
	}
}

type CreateChatCommand struct {
	NotebookId valobj.Id
}

func (h *CreateChatHandler) Handle(ctx context.Context, cmd *CreateChatCommand) (*chatentity.Chat, error) {
	userId := pkgcontext.GetUserId(ctx)
	access, err := h.notebookAccessService.Resolve(ctx, cmd.NotebookId, userId)
	if err != nil {
		return nil, errors.WithMessagef(err, "get notebook failed, notebook_id=%s", cmd.NotebookId)
	}
	if !access.CanView() {
		return nil, errors.ErrNoRecord.Msgf("notebook not found, notebook_id=%s", cmd.NotebookId)
	}

	chat, err := h.chatRepo.FindByNotebookIdAndOwnerId(ctx, cmd.NotebookId, userId)
	if err != nil {
		if !errors.Is(err, chaterrors.ErrChatNotFound) {
			return nil, errors.WithMessage(err, "find chat by notebook id and owner id failed")
		}
	}

	if err == nil {
		return chat, nil
	}

	chat = chatentity.NewChat(cmd.NotebookId, userId)
	err = h.chatRepo.Save(ctx, chat)
	if err != nil {
		return nil, errors.WithMessage(err, "save chat failed")
	}

	return chat, nil
}
