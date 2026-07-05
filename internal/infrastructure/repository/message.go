package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
	chatrepo "github.com/gonotelm-lab/gonotelm/internal/domain/chat/repository"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema/mapper"
)

type MessageRepositoryImpl struct {
	messageStore dal.ChatMessageStore
}

func NewMessageRepository(messageStore dal.ChatMessageStore) chatrepo.MessageRepository {
	return &MessageRepositoryImpl{
		messageStore: messageStore,
	}
}

var _ chatrepo.MessageRepository = &MessageRepositoryImpl{}

func (r *MessageRepositoryImpl) Save(ctx context.Context, message *entity.Message) error {
	sch, err := mapper.MessageToSchema(message)
	if err != nil {
		return err
	}

	return r.messageStore.Create(ctx, sch)
}

func (r *MessageRepositoryImpl) ListByChatId(
	ctx context.Context,
	chatId valobj.Id,
	spec chatrepo.ListSpec,
) ([]*entity.Message, error) {
	schemas, err := r.messageStore.ListByChatId(ctx, chatId, spec.Limit, spec.Offset)
	if err != nil {
		return nil, err
	}

	return mapper.MessagesFromSchema(schemas)
}

func (r *MessageRepositoryImpl) ListByChatIdBeforeSeqNo(
	ctx context.Context,
	chatId valobj.Id,
	spec chatrepo.ListByCursorSpec,
) ([]*entity.Message, error) {
	schemas, err := r.messageStore.ListByChatIdBeforeSeqNo(ctx, chatId, spec.BeforeSeqNo, spec.Limit)
	if err != nil {
		return nil, err
	}

	return mapper.MessagesFromSchema(schemas)
}
