package chat

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	chatentity "github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	notebookrepo "github.com/gonotelm-lab/gonotelm/internal/domain/notebook/repository"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type CreateChatHandler struct {
	notebookRepo notebookrepo.Repository
	chatRepo     chatrepo.Repository
}

func NewCreateChatHandler(
	notebookRepo notebookrepo.Repository,
	chatRepo chatrepo.Repository,
) *CreateChatHandler {
	return &CreateChatHandler{
		notebookRepo: notebookRepo,
		chatRepo:     chatRepo,
	}
}

type CreateChatCommand struct {
	NotebookId valobj.Id
	OwnerId    string
}

func (h *CreateChatHandler) Handle(ctx context.Context, cmd *CreateChatCommand) (*chatentity.Chat, error) {
	_, err := h.notebookRepo.FindById(ctx, cmd.NotebookId)
	if err != nil {
		return nil, errors.WithMessage(err, "find notebook by id failed")
	}

	chat, err := h.chatRepo.FindByNotebookIdAndOwnerId(ctx, cmd.NotebookId, cmd.OwnerId)
	if err != nil {
		if !errors.Is(err, chaterrors.ErrChatNotFound) {
			return nil, errors.WithMessage(err, "find chat by notebook id and owner id failed")
		}
	}

	if err == nil {
		return chat, nil
	}

	chat = chatentity.NewChat(cmd.NotebookId, cmd.OwnerId)
	err = h.chatRepo.Save(ctx, chat)
	if err != nil {
		return nil, errors.WithMessage(err, "save chat failed")
	}

	return chat, nil
}
