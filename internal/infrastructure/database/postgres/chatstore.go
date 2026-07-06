package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database"
	"github.com/gonotelm-lab/gonotelm/internal/infrastructure/database/schema"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type ChatStoreImpl struct {
	db *gorm.DB
}

var _ database.ChatStore = &ChatStoreImpl{}

func NewChatStoreImpl(db *gorm.DB) *ChatStoreImpl {
	return &ChatStoreImpl{db: db}
}

func (s *ChatStoreImpl) Create(ctx context.Context, chat *schema.Chat) error {
	if err := s.db.WithContext(ctx).Create(chat).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ChatStoreImpl) GetById(ctx context.Context, id database.Id) (*schema.Chat, error) {
	var chat schema.Chat
	err := s.db.WithContext(ctx).
		Where("id = ?", id).
		Take(&chat).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return &chat, nil
}

func (s *ChatStoreImpl) GetByNotebookIdAndOwnerId(
	ctx context.Context,
	notebookId database.Id,
	ownerId string,
) (*schema.Chat, error) {
	var chat schema.Chat
	err := s.db.WithContext(ctx).
		Where("notebook_id = ? AND owner_id = ?", notebookId, ownerId).
		Take(&chat).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return &chat, nil
}

func (s *ChatStoreImpl) ListByOwnerId(
	ctx context.Context,
	ownerId string,
	limit, offset int,
) ([]*schema.Chat, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	var rows []*schema.Chat
	err := s.db.WithContext(ctx).
		Where("owner_id = ?", ownerId).
		Order("updated_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ChatStoreImpl) ListByNotebookId(
	ctx context.Context,
	notebookId database.Id,
) ([]*schema.Chat, error) {
	var rows []*schema.Chat
	err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Find(&rows).Error
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ChatStoreImpl) DeleteById(ctx context.Context, id database.Id) error {
	if err := s.db.WithContext(ctx).Where("id = ?", id).Delete(&schema.Chat{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}

func (s *ChatStoreImpl) DeleteByNotebookId(ctx context.Context, notebookId database.Id) error {
	if err := s.db.WithContext(ctx).
		Where("notebook_id = ?", notebookId).
		Delete(&schema.Chat{}).Error; err != nil {
		return sql.WrapErr(err)
	}
	return nil
}
