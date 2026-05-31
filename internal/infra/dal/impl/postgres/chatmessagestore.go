package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	"github.com/gonotelm-lab/gonotelm/pkg/batch"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"
	"github.com/gonotelm-lab/gonotelm/pkg/uuid"

	"gorm.io/gorm"
)

type ChatMessageStoreImpl struct {
	db *gorm.DB
}

var chatIDsDeleteBatchSize = 1000

var _ dal.ChatMessageStore = &ChatMessageStoreImpl{}

func NewChatMessageStoreImpl(db *gorm.DB) *ChatMessageStoreImpl {
	return &ChatMessageStoreImpl{db: db}
}

func (s *ChatMessageStoreImpl) Create(ctx context.Context, message *schema.ChatMessage) error {
	if message.Id.IsZero() {
		message.Id = uuid.NewV7()
	}

	if err := s.db.WithContext(ctx).Create(message).Error; err != nil {
		return sql.WrapErr(err)
	}

	return nil
}

func (s *ChatMessageStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.ChatMessage, error) {
	var row schema.ChatMessage
	err := s.db.WithContext(ctx).
		Where("id = ?", id).
		Take(&row).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &row, nil
}

func (s *ChatMessageStoreImpl) GetByIdAndChatId(
	ctx context.Context,
	id dal.Id,
	chatId dal.Id,
) (*schema.ChatMessage, error) {
	var row schema.ChatMessage
	err := s.db.WithContext(ctx).
		Where("id = ? AND chat_id = ?", id, chatId).
		Take(&row).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return &row, nil
}

func (s *ChatMessageStoreImpl) ListByChatId(
	ctx context.Context,
	chatId dal.Id,
	limit, offset int,
) ([]*schema.ChatMessage, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	var rows []*schema.ChatMessage
	err := s.db.WithContext(ctx).
		Where("chat_id = ?", chatId).
		Order("seq_no DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *ChatMessageStoreImpl) ListByChatIdBeforeSeqNo(
	ctx context.Context,
	chatId dal.Id,
	beforeSeqNo int64,
	limit int,
) ([]*schema.ChatMessage, error) {
	if limit <= 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d", limit)
	}
	if beforeSeqNo <= 0 {
		return nil, xerror.ErrParams.Msgf("invalid cursor params: before_seq_no=%d", beforeSeqNo)
	}

	var rows []*schema.ChatMessage
	err := s.db.WithContext(ctx).
		Where("chat_id = ? AND seq_no < ?", chatId, beforeSeqNo).
		Order("seq_no DESC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}

	return rows, nil
}

func (s *ChatMessageStoreImpl) DeleteByChatId(ctx context.Context, chatId dal.Id) error {
	return s.BatchDeleteByChatIds(ctx, []dal.Id{chatId})
}

func (s *ChatMessageStoreImpl) BatchDeleteByChatIds(ctx context.Context, chatIds []dal.Id) error {
	if len(chatIds) == 0 {
		return nil
	}

	_, err := batch.BatchMap(
		ctx,
		chatIds,
		chatIDsDeleteBatchSize,
		func(ctx context.Context, batchChatIDs []dal.Id) ([]struct{}, error) {
			if err := s.db.WithContext(ctx).
				Where("chat_id IN ?", batchChatIDs).
				Delete(&schema.ChatMessage{}).Error; err != nil {
				return nil, sql.WrapErr(err)
			}
			return nil, nil
		},
	)
	if err != nil {
		return err
	}

	return nil
}
