package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
)

type MessageRepository interface {
	Save(ctx context.Context, message *entity.Message) error
	ListByChatId(ctx context.Context, chatId valobj.Id, spec ListSpec) ([]*entity.Message, error)
}

type ContextMessageRepository interface {
	Append(ctx context.Context, chatId valobj.Id, messages []*entity.ContextMessage) error
	Destroy(ctx context.Context, chatId valobj.Id) error
	BatchDestroy(ctx context.Context, chatIds []valobj.Id) error
	ListAll(ctx context.Context, chatId valobj.Id) ([]*entity.ContextMessage, error)

	// override existing messages
	Set(ctx context.Context, chatId valobj.Id, messages []*entity.ContextMessage) error
}
