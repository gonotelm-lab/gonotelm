package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"gorm.io/gorm"
)

type ChatMessageStoreImpl struct {
	db *gorm.DB
}

var _ dal.ChatMessageStore = &ChatMessageStoreImpl{}

func NewChatMessageStoreImpl(db *gorm.DB) *ChatMessageStoreImpl {
	return &ChatMessageStoreImpl{db: db}
}

func (s *ChatMessageStoreImpl) Create(ctx context.Context, message *schema.ChatMessage) error {
	if message.Id.IsZero() {
		message.Id = uuid.NewV7()
	}

	tx := s.db.WithContext(ctx).Exec(
		"INSERT INTO chat_messages (id, chat_id, user_id, msg_type, content, seq_no, extra) VALUES (?, ?, ?, ?, ?, ?, ?)",
		message.Id,
		message.ChatId,
		message.UserId,
		message.MsgType,
		message.Content,
		message.SeqNo,
		message.Extra,
	)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}

func (s *ChatMessageStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.ChatMessage, error) {
	row, err := gorm.G[*schema.ChatMessage](s.db).
		Raw(
			"SELECT id, chat_id, user_id, msg_type, content, seq_no, extra FROM chat_messages WHERE id = ? LIMIT 1",
			id,
		).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return row, nil
}

func (s *ChatMessageStoreImpl) GetByIdAndChatId(
	ctx context.Context,
	id dal.Id,
	chatId dal.Id,
) (*schema.ChatMessage, error) {
	row, err := gorm.G[*schema.ChatMessage](s.db).
		Raw(
			"SELECT id, chat_id, user_id, msg_type, content, seq_no, extra FROM chat_messages WHERE id = ? AND chat_id = ? LIMIT 1",
			id,
			chatId,
		).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return row, nil
}

func (s *ChatMessageStoreImpl) ListByChatId(
	ctx context.Context,
	chatId dal.Id,
	limit, offset int,
) ([]*schema.ChatMessage, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	rows, err := gorm.G[*schema.ChatMessage](s.db).
		Raw(
			"SELECT id, chat_id, user_id, msg_type, content, seq_no, extra FROM chat_messages WHERE chat_id = ? ORDER BY seq_no DESC LIMIT ? OFFSET ?",
			chatId,
			limit,
			offset,
		).
		Find(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *ChatMessageStoreImpl) DeleteByChatId(ctx context.Context, chatId dal.Id) error {
	tx := s.db.WithContext(ctx).Exec("DELETE FROM chat_messages WHERE chat_id = ?", chatId)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
}
