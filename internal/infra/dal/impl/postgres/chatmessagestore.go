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
		"INSERT INTO chat_messages (id, chat_id, user_id, role, content, seq_no) VALUES (?, ?, ?, ?, ?, ?)",
		message.Id,
		message.ChatId,
		message.UserId,
		message.Role,
		message.Content,
		message.SeqNo,
	)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}

	return nil
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
			"SELECT id, chat_id, user_id, role, content, seq_no FROM chat_messages WHERE chat_id = ? ORDER BY seq_no ASC LIMIT ? OFFSET ?",
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
