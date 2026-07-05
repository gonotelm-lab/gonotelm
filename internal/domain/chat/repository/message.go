package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
)

type ListByCursorSpec struct {
	BeforeSeqNo int64
	Limit       int
}

type MessageRepository interface {
	Save(ctx context.Context, message *entity.Message) error
	ListByChatId(ctx context.Context, chatId valobj.Id, spec ListSpec) ([]*entity.Message, error)
	ListByChatIdBeforeSeqNo(ctx context.Context, chatId valobj.Id, spec ListByCursorSpec) ([]*entity.Message, error)
	DeleteByChatIds(ctx context.Context, chatIds []valobj.Id) error
}

type ContextMessageRepository interface {
	Append(ctx context.Context, chatId valobj.Id, messages []*entity.ContextMessage) error
	Destroy(ctx context.Context, chatId valobj.Id) error
	BatchDestroy(ctx context.Context, chatIds []valobj.Id) error
	ListAll(ctx context.Context, chatId valobj.Id) ([]*entity.ContextMessage, error)

	// override existing messages
	Set(ctx context.Context, chatId valobj.Id, messages []*entity.ContextMessage) error
}
