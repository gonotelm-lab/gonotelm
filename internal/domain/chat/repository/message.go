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
	FindByChatIdMsgId(ctx context.Context, chatId, msgId valobj.Id) (*entity.Message, error)
	ListByChatId(ctx context.Context, chatId valobj.Id, spec ListSpec) ([]*entity.Message, error)
	ListByChatIdBeforeSeqNo(ctx context.Context, chatId valobj.Id, spec ListByCursorSpec) ([]*entity.Message, error)
	DeleteByChatIds(ctx context.Context, chatIds []valobj.Id) error
}

type ContextMessageRepository interface {
	Append(ctx context.Context, chatId valobj.Id, messages []*entity.ContextMessage) error
	Destroy(ctx context.Context, chatId valobj.Id) error
	BatchDestroy(ctx context.Context, chatIds []valobj.Id) error
	ListAll(ctx context.Context, chatId valobj.Id) ([]*entity.ContextMessage, error)
	// 从start开始获取limit条 start从0开始
	List(ctx context.Context, chatId valobj.Id, start, limit int) ([]*entity.ContextMessage, error)
	// 获取最近的limit条
	ListRecent(ctx context.Context, chatId valobj.Id, limit int) ([]*entity.ContextMessage, error)

	// override existing messages
	Set(ctx context.Context, chatId valobj.Id, messages []*entity.ContextMessage) error
}
