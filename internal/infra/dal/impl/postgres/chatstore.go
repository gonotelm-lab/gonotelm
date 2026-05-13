package postgres

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/infra/dal"
	"github.com/gonotelm-lab/gonotelm/internal/infra/dal/schema"
	xerror "github.com/gonotelm-lab/gonotelm/pkg/errors"
	"github.com/gonotelm-lab/gonotelm/pkg/sql"

	"gorm.io/gorm"
)

type ChatStoreImpl struct {
	db *gorm.DB
}

var _ dal.ChatStore = &ChatStoreImpl{}

func NewChatStoreImpl(db *gorm.DB) *ChatStoreImpl {
	return &ChatStoreImpl{db: db}
}

func (s *ChatStoreImpl) Create(ctx context.Context, chat *schema.Chat) error {
	tx := s.db.WithContext(ctx).Exec(
		"INSERT INTO chats (id, notebook_id, owner_id, updated_at) VALUES (?, ?, ?, ?)",
		chat.Id, chat.NotebookId, chat.OwnerId, chat.UpdatedAt,
	)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}
	return nil
}

func (s *ChatStoreImpl) GetById(ctx context.Context, id dal.Id) (*schema.Chat, error) {
	chat, err := gorm.G[*schema.Chat](s.db).
		Raw("SELECT id, notebook_id, owner_id, updated_at FROM chats WHERE id = ? LIMIT 1", id).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return chat, nil
}

func (s *ChatStoreImpl) GetByNotebookIdAndOwnerId(
	ctx context.Context,
	notebookId dal.Id,
	ownerId string,
) (*schema.Chat, error) {
	chat, err := gorm.G[*schema.Chat](s.db).
		Raw("SELECT id, notebook_id, owner_id, updated_at FROM chats WHERE notebook_id = ? AND owner_id = ? LIMIT 1",
			notebookId, ownerId).
		First(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return chat, nil
}

func (s *ChatStoreImpl) ListByOwnerId(
	ctx context.Context,
	ownerId string,
	limit, offset int,
) ([]*schema.Chat, error) {
	if limit <= 0 || offset < 0 {
		return nil, xerror.ErrParams.Msgf("invalid pagination params: limit=%d offset=%d", limit, offset)
	}

	rows, err := gorm.G[*schema.Chat](s.db).
		Raw("SELECT id, notebook_id, owner_id, updated_at FROM chats WHERE owner_id = ? ORDER BY updated_at DESC LIMIT ? OFFSET ?",
			ownerId, limit, offset).
		Find(ctx)
	if err != nil {
		return nil, sql.WrapErr(err)
	}
	return rows, nil
}

func (s *ChatStoreImpl) DeleteById(ctx context.Context, id dal.Id) error {
	tx := s.db.WithContext(ctx).Exec("DELETE FROM chats WHERE id = ?", id)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}
	return nil
}

func (s *ChatStoreImpl) DeleteByNotebookId(ctx context.Context, notebookId dal.Id) error {
	tx := s.db.WithContext(ctx).Exec("DELETE FROM chats WHERE notebook_id = ?", notebookId)
	if tx.Error != nil {
		return sql.WrapErr(tx.Error)
	}
	return nil
}
