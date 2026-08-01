package repository

import (
	"context"

	"github.com/gonotelm-lab/gonotelm/internal/core/valobj"
	"github.com/gonotelm-lab/gonotelm/internal/domain/chat/entity"
)

// 聊天中的建议问题
type SuggestionRepository interface {
	Get(ctx context.Context, chatId valobj.Id) (*entity.Suggestion, error)
	Save(ctx context.Context, chatId valobj.Id, suggestion *entity.Suggestion) error
	Delete(ctx context.Context, chatId valobj.Id) error
}
