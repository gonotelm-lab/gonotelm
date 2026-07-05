package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chaterrors "github.com/gonotelm-lab/gonotelm/internal/domain/chat/errors"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema/mapper"
	"github.com/gonotelm-lab/gonotelm/pkg/errors"
)

type ChatRepositoryImpl struct {
	chatStore dal.ChatStore
}

func NewChatRepository(chatStore dal.ChatStore) repository.Repository {
	return &ChatRepositoryImpl{chatStore: chatStore}
}

var _ repository.Repository = &ChatRepositoryImpl{}

func (r *ChatRepositoryImpl) Save(ctx context.Context, chat *entity.Chat) error {
	if chat.IsDeleted() {
		return r.chatStore.DeleteById(ctx, chat.Id)
	}

	sch := mapper.ChatToSchema(chat)
	err := r.chatStore.Create(ctx, sch)
	if err != nil {
		return err
	}

	return nil
}

func (r *ChatRepositoryImpl) FindById(ctx context.Context, id valobj.Id) (*entity.Chat, error) {
	sch, err := r.chatStore.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, chaterrors.ErrChatNotFound
		}
		return nil, err
	}

	return mapper.ChatFromSchema(sch), nil
}

func (r *ChatRepositoryImpl) FindByNotebookIdAndOwnerId(ctx context.Context, notebookId valobj.Id, ownerId string) (*entity.Chat, error) {
	sch, err := r.chatStore.GetByNotebookIdAndOwnerId(ctx, notebookId, ownerId)
	if err != nil {
		if errors.Is(err, errors.ErrNoRecord) {
			return nil, chaterrors.ErrChatNotFound
		}
		return nil, err
	}

	return mapper.ChatFromSchema(sch), nil
}

func (r *ChatRepositoryImpl) ListByNotebookId(
	ctx context.Context,
	notebookId valobj.Id,
) ([]*entity.Chat, error) {
	schemas, err := r.chatStore.ListByNotebookId(ctx, notebookId)
	if err != nil {
		return nil, err
	}

	return mapper.ChatsFromSchema(schemas), nil
}

func (r *ChatRepositoryImpl) DeleteByNotebookId(ctx context.Context, notebookId valobj.Id) error {
	return r.chatStore.DeleteByNotebookId(ctx, notebookId)
}
